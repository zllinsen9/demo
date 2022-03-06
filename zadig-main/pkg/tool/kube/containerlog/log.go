/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package containerlog

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/pkg/tool/log"
)

func GetContainerLogs(namespace, podName, containerName string, follow bool, tailLines int64, out io.Writer, clientset kubernetes.Interface) error {
	readCloser, err := GetContainerLogStream(context.TODO(), namespace, podName, containerName, follow, tailLines, clientset)
	if err != nil {
		log.Warnf("Failed to get pod log from stream: %s. Try to get logs from pod object.", err)

		// For Serverless K8s, we may not be able to get logs from Pod in the Failed state, but we can configure
		// `container.terminationMessagePolicy=FallbackToLogsOnError` to get the latest exception information from the Pod Object.
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pod %s in %s: %s", podName, namespace, err)
		}

		_, err = out.Write([]byte(pod.Status.ContainerStatuses[0].State.Terminated.Message))

		return err
	}

	defer func() {
		_ = readCloser.Close()
	}()

	_, err = io.Copy(out, readCloser)
	return err
}

func GetContainerLogStream(ctx context.Context, namespace, podName, containerName string, follow bool, tailLines int64, clientset kubernetes.Interface) (io.ReadCloser, error) {
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		Follow:    follow,
	}

	if tailLines > 0 {
		logOptions.TailLines = &tailLines
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	return req.Stream(ctx)
}
