package workflow

import (
	"bufio"
	"bytes"
	"fmt"
	"hash/fnv"
	"math"
	"sync"
	"time"

	"github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo/pkg/client/clientset/versioned"
	wfinformers "github.com/argoproj/argo/pkg/client/informers/externalversions"
	"github.com/argoproj/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// LogEntry struct
type LogEntry struct {
	DisplayName string
	Pod         string
	Time        time.Time
	Line        string
}

// NewLogPrinter getlogs
func NewLogPrinter(kubeClient kubernetes.Interface, follow bool, sinceTime *metav1.Time) *LogPrinter {
	return &LogPrinter{
		kubeClient: kubeClient,
		follow:     follow,
		sinceTime:  sinceTime,
		container:  "main",
	}
}

// LogPrinter struct
type LogPrinter struct {
	container    string
	follow       bool
	sinceSeconds *int64
	sinceTime    *metav1.Time
	tail         *int64
	timestamps   bool
	kubeClient   kubernetes.Interface
}

//GetWorkflowLogs logs
func (p *LogPrinter) GetWorkflowLogs(wf *v1alpha1.Workflow) map[string]bytes.Buffer {

	logEntries := p.PrintWorkflowLogs(wf)

	bufferMap := make(map[string]bytes.Buffer)

	for _, logEntry := range logEntries {
		buffer := bufferMap[logEntry.Pod]
		buffer.WriteString(logEntry.Line)
		buffer.WriteString("\n")
		bufferMap[logEntry.Pod] = buffer
	}
	return bufferMap
}

// PrintWorkflowLogs prints logs for all workflow pods
func (p *LogPrinter) PrintWorkflowLogs(wf *v1alpha1.Workflow) []LogEntry {
	logEntries := p.printRecentWorkflowLogs(wf)
	// if p.follow && wf.Status.Phase == v1alpha1.NodeRunning {
	// 	p.printLiveWorkflowLogs(wf, timeByPod)
	// }
	return logEntries
}

// PrintPodLogs prints logs for a single pod
func (p *LogPrinter) PrintPodLogs(podName string) error {
	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return err
	}
	var logs []LogEntry
	err = p.getPodLogs("", podName, namespace, p.follow, p.tail, p.sinceSeconds, p.sinceTime, func(entry LogEntry) {
		logs = append(logs, entry)
	})
	if err != nil {
		return err
	}
	for _, entry := range logs {
		p.printLogEntry(entry)
	}
	return nil
}

// Prints logs for workflow pod steps and return most recent log timestamp per pod name
func (p *LogPrinter) printRecentWorkflowLogs(wf *v1alpha1.Workflow) []LogEntry {
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
			err := p.getPodLogs(getDisplayName(node), node.ID, wf.Namespace, false, p.tail, p.sinceSeconds, p.sinceTime, func(entry LogEntry) {
				podLogs = append(podLogs, entry)
			})

			if err != nil {
				log.Warn(err)
				return
			}

			mux.Lock()
			logs = append(logs, podLogs)
			mux.Unlock()
		}()

	}
	wg.Wait()

	flattenLogs := mergeSorted(logs)

	if p.tail != nil {
		tail := *p.tail
		if int64(len(flattenLogs)) < tail {
			tail = int64(len(flattenLogs))
		}
		flattenLogs = flattenLogs[0:tail]
	}
	return flattenLogs
}

func (p *LogPrinter) setupWorkflowInformer(namespace string, name string, callback func(wf *v1alpha1.Workflow, done bool)) cache.SharedIndexInformer {
	wfcClientset := wfclientset.NewForConfigOrDie(restConfig)
	wfInformerFactory := wfinformers.NewFilteredSharedInformerFactory(wfcClientset, 20*time.Minute, namespace, nil)
	informer := wfInformerFactory.Argoproj().V1alpha1().Workflows().Informer()
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(old, new interface{}) {
				updatedWf := new.(*v1alpha1.Workflow)
				if updatedWf.Name == name {
					callback(updatedWf, updatedWf.Status.Phase != v1alpha1.NodeRunning)
				}
			},
			DeleteFunc: func(obj interface{}) {
				deletedWf := obj.(*v1alpha1.Workflow)
				if deletedWf.Name == name {
					callback(deletedWf, true)
				}
			},
		},
	)
	return informer
}

// Prints live logs for workflow pods, starting from time specified in timeByPod name.
func (p *LogPrinter) printLiveWorkflowLogs(workflow *v1alpha1.Workflow, timeByPod map[string]*time.Time) {
	logs := make(chan LogEntry)
	streamedPods := make(map[string]bool)

	processPods := func(wf *v1alpha1.Workflow) {
		for id := range wf.Status.Nodes {
			node := wf.Status.Nodes[id]
			if node.Type == v1alpha1.NodeTypePod && node.Phase != v1alpha1.NodeError && streamedPods[node.ID] == false {
				streamedPods[node.ID] = true
				go func() {
					var sinceTimePtr *metav1.Time
					podTime := timeByPod[node.ID]
					if podTime != nil {
						sinceTime := metav1.NewTime(podTime.Add(time.Second))
						sinceTimePtr = &sinceTime
					}
					err := p.getPodLogs(getDisplayName(node), node.ID, wf.Namespace, true, nil, nil, sinceTimePtr, func(entry LogEntry) {
						logs <- entry
					})
					if err != nil {
						log.Warn(err)
					}
				}()
			}
		}
	}

	processPods(workflow)
	informer := p.setupWorkflowInformer(workflow.Namespace, workflow.Name, func(wf *v1alpha1.Workflow, done bool) {
		if done {
			close(logs)
		} else {
			processPods(wf)
		}
	})

	stopChannel := make(chan struct{})
	go func() {
		informer.Run(stopChannel)
	}()
	defer close(stopChannel)

	for entry := range logs {
		p.printLogEntry(entry)
	}
}

func getDisplayName(node v1alpha1.NodeStatus) string {
	res := node.DisplayName
	if res == "" {
		res = node.Name
	}
	return res
}

func (p *LogPrinter) printLogEntry(entry LogEntry) {
	line := entry.Line
	if p.timestamps {
		line = entry.Time.Format(time.RFC3339) + "	" + line
	}
	if entry.DisplayName != "" {
		colors := []int{FgRed, FgGreen, FgYellow, FgBlue, FgMagenta, FgCyan, FgWhite, FgDefault}
		h := fnv.New32a()
		_, err := h.Write([]byte(entry.DisplayName))
		errors.CheckError(err)
		colorIndex := int(math.Mod(float64(h.Sum32()), float64(len(colors))))
		line = ansiFormat(entry.DisplayName, colors[colorIndex]) + ":	" + line
	}
	fmt.Println(line)
}

func (p *LogPrinter) ensureContainerStarted(podName string, podNamespace string, container string, retryCnt int, retryTimeout time.Duration) error {
	for retryCnt > 0 {
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
			time.Sleep(retryTimeout)
			retryCnt--
		} else {
			return nil
		}
	}
	return fmt.Errorf("container '%s' of pod '%s' has not started within expected timeout", container, podName)
}

func (p *LogPrinter) getPodLogs(
	DisplayName string, podName string, podNamespace string, follow bool, tail *int64, sinceSeconds *int64, sinceTime *metav1.Time, callback func(entry LogEntry)) error {
	err := p.ensureContainerStarted(podName, podNamespace, p.container, 2, time.Second)
	if err != nil {
		return err
	}
	stream, err := p.kubeClient.CoreV1().Pods(podNamespace).GetLogs(podName, &v1.PodLogOptions{
		Container:    p.container,
		Follow:       follow,
		Timestamps:   false,
		SinceSeconds: sinceSeconds,
		SinceTime:    sinceTime,
		TailLines:    tail,
	}).Stream()
	if err == nil {
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Text()

			callback(LogEntry{
				Pod:         podName,
				DisplayName: DisplayName,
				// Time:        logTime,
				Line: line,
			})
			// parts := strings.Split(line, " ")
			// logTime, err := time.Parse(time.RFC3339, parts[0])
			// if err == nil {
			// 	lines := strings.Join(parts[1:], " ")
			// 	for _, line := range strings.Split(lines, "\r") {
			// 		if line != "" {
			// 			callback(LogEntry{
			// 				Pod:         podName,
			// 				DisplayName: DisplayName,
			// 				Time:        logTime,
			// 				Line:        line,
			// 			})
			// 		}
			// 	}
			// }
		}
	}
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
