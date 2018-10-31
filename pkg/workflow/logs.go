package workflow

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/kubebuild/agent/pkg/graphql"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LogEntry struct
type LogEntry struct {
	DisplayName string
	Pod         string
	Container   string
	Time        time.Time
	Line        string
}

// NewLogUploader getlogs
func NewLogUploader(cluster graphql.Cluster, kubeClient kubernetes.Interface, log *log.Logger) *LogUploader {
	return &LogUploader{
		kubeClient: kubeClient,
		log:        log,
		cluster:    cluster,
	}
}

// LogUploader struct
type LogUploader struct {
	kubeClient kubernetes.Interface
	log        *log.Logger
	cluster    graphql.Cluster
}

// BufferDelim used to delim container and pod
var BufferDelim = "||"

// UploadWorkflowLogs upload wf logs
func (p *LogUploader) UploadWorkflowLogs(wf *v1alpha1.Workflow, build graphql.RunningBuild, shaMap map[string]string, mutex *sync.Mutex) {
	var requestGroup singleflight.Group

	bufferMap := p.GetWorkflowLogs(wf)

	creds := credentials.Value{
		AccessKeyID:     string(build.Cluster.LogAwsKey),
		SecretAccessKey: string(build.Cluster.LogAwsSecret),
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(string(build.LogRegion)),
		Credentials: credentials.NewStaticCredentialsFromCreds(creds),
	}))

	bucket := fmt.Sprintf("kubebuild-logs-%s", build.LogRegion)
	svc := s3manager.NewUploader(sess)

	for k, v := range bufferMap {
		readBytes, err := ioutil.ReadAll(&v)
		if err != nil {
			p.log.WithError(err).Error("cannot read buffer")
		}
		h := sha256.New()

		h.Write(readBytes)
		bs := hex.EncodeToString(h.Sum(nil))

		if shaMap[k] == bs {
			p.log.WithField("container", k).Debug("skipping same shas")
			continue
		}
		mutex.Lock()
		shaMap[k] = bs
		mutex.Unlock()

		requestGroup.Do(k, func() (interface{}, error) {
			return p.uploadToS3(k, svc, build, readBytes, bucket)
		})
		if err != nil {
			p.log.WithError(err).Error("channel failed")
		}
	}
}

func (p *LogUploader) uploadToS3(k string, svc *s3manager.Uploader, build graphql.RunningBuild, readBytes []byte, bucket string) (*s3manager.UploadOutput, error) {
	s := strings.Split(k, BufferDelim)
	podName, container := s[0], s[1]
	var gZipBuffer bytes.Buffer

	w := gzip.NewWriter(&gZipBuffer)
	_, err := w.Write(readBytes)

	if err != nil {
		p.log.WithError(err).Error("cannot gzip file")
	}
	w.Close()

	key := fmt.Sprintf("%s/%s/%s/%s", p.cluster.Name, build.ID, podName, container)
	uploadOutput, err := svc.Upload(&s3manager.UploadInput{
		ACL:             aws.String("public-read"),
		CacheControl:    aws.String("no-cache"),
		ContentType:     aws.String("text/plain; charset=utf-8"),
		ContentEncoding: aws.String("gzip"),
		Expires:         aws.Time(time.Now().AddDate(0, 1, 0)),
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		Body:            &gZipBuffer,
	})

	if err != nil {
		p.log.WithError(err).Error("failed to upload logs")
	}
	p.log.Debug(uploadOutput)
	return uploadOutput, err

}

//GetWorkflowLogs logs
func (p *LogUploader) GetWorkflowLogs(wf *v1alpha1.Workflow) map[string]bytes.Buffer {

	logEntries := p.getRecentWfLogs(wf)

	bufferMap := make(map[string]bytes.Buffer)

	for _, logEntry := range logEntries {
		bufferKey := fmt.Sprintf("%s%s%s", logEntry.Pod, BufferDelim, logEntry.Container)
		buffer := bufferMap[bufferKey]
		buffer.WriteString(logEntry.Line)
		buffer.WriteString("\n")
		bufferMap[bufferKey] = buffer
	}

	return bufferMap
}

// Prints logs for workflow pod steps and return most recent log timestamp per pod name
func (p *LogUploader) getRecentWfLogs(wf *v1alpha1.Workflow) []LogEntry {
	var podNodes []v1alpha1.NodeStatus
	for _, node := range wf.Status.Nodes {
		if node.Type == v1alpha1.NodeTypePod && node.Phase != v1alpha1.NodeError {
			podNodes = append(podNodes, node)
		}
	}
	var logs [][]LogEntry
	var wg sync.WaitGroup
	wg.Add(len(podNodes))
	var mux sync.Mutex

	for i := range podNodes {
		node := podNodes[i]
		go func() {
			defer wg.Done()
			var podLogs []LogEntry
			err := p.getPodLogs(getDisplayName(node), node.ID, wf.Namespace, func(entry LogEntry) {
				podLogs = append(podLogs, entry)
			})

			if err != nil {
				p.log.Warn(err)
				return
			}

			mux.Lock()
			logs = append(logs, podLogs)
			mux.Unlock()
		}()

	}
	wg.Wait()

	flattenLogs := mergeSorted(logs)

	return flattenLogs
}

func getDisplayName(node v1alpha1.NodeStatus) string {
	res := node.DisplayName
	if res == "" {
		res = node.Name
	}
	return res
}

func (p *LogUploader) ensureContainerStarted(podName string, podNamespace string, container string, retryCnt int, retryTimeout time.Duration) error {
	// for retryCnt > 0 {
	pod, err := p.kubeClient.CoreV1().Pods(podNamespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var containerStatus *v1.ContainerStatus
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == container {
			containerStatus = &status
			break
		}
	}
	if containerStatus == nil || containerStatus.State.Waiting != nil {
		// time.Sleep(retryTimeout)
		// retryCnt--
		return fmt.Errorf("container '%s' of pod '%s' has not started within expected timeout", container, podName)
	} else {
		return nil
	}
	// }
	// return fmt.Errorf("container '%s' of pod '%s' has not started within expected timeout", container, podName)
}

func (p *LogUploader) getPodLogs(
	DisplayName string, podName string, podNamespace string, callback func(entry LogEntry)) error {
	pod, err := p.kubeClient.CoreV1().Pods(podNamespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(pod.Spec.Containers))

	containerStatusMap := make(map[string]v1.ContainerStatus)
	for _, status := range pod.Status.ContainerStatuses {
		containerStatusMap[status.Name] = status
	}

	for i := range pod.Spec.Containers {
		container := pod.Spec.Containers[i]
		go func() {
			defer wg.Done()
			containerStatus := containerStatusMap[container.Name]
			if containerStatus.State.Waiting != nil {
				return
			}
			p.log.WithField("container", container.Name).Debug("getting logs")

			stream, err := p.kubeClient.CoreV1().Pods(podNamespace).GetLogs(podName, &v1.PodLogOptions{
				Container:  container.Name,
				Timestamps: false,
			}).Stream()

			if err == nil {
				scanner := bufio.NewScanner(stream)
				for scanner.Scan() {
					line := scanner.Text()

					callback(LogEntry{
						Pod:         podName,
						DisplayName: DisplayName,
						Container:   container.Name,
						Line:        line,
					})
				}
			}
		}()
	}
	wg.Wait()
	return err
}

func mergeSorted(logs [][]LogEntry) []LogEntry {
	if len(logs) == 0 {
		return make([]LogEntry, 0)
	}
	for len(logs) > 1 {
		left := logs[0]
		right := logs[1]
		size, i, j := len(left)+len(right), 0, 0
		merged := make([]LogEntry, size, size)

		for k := 0; k < size; k++ {
			if i > len(left)-1 && j <= len(right)-1 {
				merged[k] = right[j]
				j++
			} else if j > len(right)-1 && i <= len(left)-1 {
				merged[k] = left[i]
				i++
			} else if left[i].Time.Before(right[j].Time) {
				merged[k] = left[i]
				i++
			} else {
				merged[k] = right[j]
				j++
			}
		}
		logs = append(logs[2:], merged)
	}
	return logs[0]
}
