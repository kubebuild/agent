package workflow

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo/pkg/client/clientset/versioned"
	"github.com/argoproj/argo/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/kubebuild/agent/pkg/graphql"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Global variables
var (
	restConfig       *rest.Config
	clientConfig     clientcmd.ClientConfig
	clientset        *kubernetes.Clientset
	kubeClient       kubernetes.Interface
	wfClient         v1alpha1.WorkflowInterface
	jobStatusIconMap map[wfv1.NodePhase]string
	noColor          bool
)

// ANSI escape codes
const (
	escape    = "\x1b"
	noFormat  = 0
	Bold      = 1
	FgBlack   = 30
	FgRed     = 31
	FgGreen   = 32
	FgYellow  = 33
	FgBlue    = 34
	FgMagenta = 35
	FgCyan    = 36
	FgWhite   = 37
	FgDefault = 39
)

func initializeSession() {
	jobStatusIconMap = map[wfv1.NodePhase]string{
		wfv1.NodePending:   ansiFormat("◷", FgYellow),
		wfv1.NodeRunning:   ansiFormat("●", FgCyan),
		wfv1.NodeSucceeded: ansiFormat("✔", FgGreen),
		wfv1.NodeSkipped:   ansiFormat("○", FgDefault),
		wfv1.NodeFailed:    ansiFormat("✖", FgRed),
		wfv1.NodeError:     ansiFormat("⚠", FgRed),
	}
}

func initKubeClient(inCluster bool, kubectlPath string, log *logrus.Logger) *kubernetes.Clientset {
	if clientset != nil {
		return clientset
	}
	var err error
	var config clientcmdapi.Config

	if !inCluster && kubectlPath != "" {
		c, err := clientcmd.LoadFromFile(kubectlPath)
		if err != nil {
			log.Fatal(err)
		}
		config = *c
		overrides := &clientcmd.ConfigOverrides{}
		clientConfig = clientcmd.NewDefaultClientConfig(config, overrides)
		restConfig, err = clientConfig.ClientConfig()
	} else {
		// Config in cluster
		restConfig, err = clientcmd.DefaultClientConfig.ClientConfig()
	}
	if err != nil {
		log.Fatal(err)
	}

	// create the clientset
	clientset, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatal(err)
	}
	return clientset
}

// InitWorkflowClient creates a new client for the Kubernetes Workflow CRD.
func InitWorkflowClient(cluster graphql.Cluster, inCluster bool, kubectlPath string, log *logrus.Logger) (v1alpha1.WorkflowInterface, kubernetes.Interface) {
	if wfClient != nil && kubeClient != nil {
		return wfClient, kubeClient
	}
	initKubeClient(inCluster, kubectlPath, log)
	var namespace string
	namespace = string(cluster.Name)
	wfcs := wfclientset.NewForConfigOrDie(restConfig)
	kubeClient = kubernetes.NewForConfigOrDie(restConfig)
	wfClient = wfcs.ArgoprojV1alpha1().Workflows(namespace)
	return wfClient.(v1alpha1.WorkflowInterface), kubeClient
}

// ansiFormat wraps ANSI escape codes to a string to format the string to a desired color.
// NOTE: we still apply formatting even if there is no color formatting desired.
// The purpose of doing this is because when we apply ANSI color escape sequences to our
// output, this confuses the tabwriter library which miscalculates widths of columns and
// misaligns columns. By always applying a ANSI escape sequence (even when we don't want
// color, it provides more consistent string lengths so that tabwriter can calculate
// widths correctly.
func ansiFormat(s string, codes ...int) string {
	if noColor || os.Getenv("TERM") == "dumb" || len(codes) == 0 {
		return s
	}
	codeStrs := make([]string, len(codes))
	for i, code := range codes {
		codeStrs[i] = strconv.Itoa(code)
	}
	sequence := strings.Join(codeStrs, ";")
	return fmt.Sprintf("%s[%sm%s%s[%dm", escape, sequence, s, escape, noFormat)
}
