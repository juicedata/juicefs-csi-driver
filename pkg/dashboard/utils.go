/*
 Copyright 2023 Juicedata Inc

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

package dashboard

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"

	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

type ReverseSort struct {
	sort.Interface
}

func (r *ReverseSort) Less(i, j int) bool {
	return !r.Interface.Less(i, j)
}

func Reverse(data sort.Interface) sort.Interface {
	return &ReverseSort{data}
}

func LabelSelectorOfMount(pv corev1.PersistentVolume) labels.Selector {
	values := []string{pv.Spec.CSI.VolumeHandle}
	if pv.Spec.StorageClassName != "" {
		values = append(values, pv.Spec.StorageClassName)
	}
	sl := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      common.PodUniqueIdLabelKey,
			Operator: metav1.LabelSelectorOpIn,
			Values:   values,
		}},
	}
	labelMap, _ := metav1.LabelSelectorAsSelector(&sl)
	return labelMap
}

func isShareMount(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "STORAGE_CLASS_SHARE_MOUNT" && env.Value == "true" {
			return true
		}
	}

	return false
}

func SetJobAsConfigMapOwner(cm *corev1.ConfigMap, owner *batchv1.Job) {
	controller := true
	cm.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       owner.Name,
		UID:        owner.UID,
		Controller: &controller,
	}})
}

func getUniqueIdFromSecretName(secretName string) string {
	re := regexp.MustCompile(`juicefs-(.*?)-secret`)
	match := re.FindStringSubmatch(secretName)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func IsPVCSelectorEmpty(selector *config.PVCSelector) bool {
	if selector == nil {
		return true
	}

	return reflect.DeepEqual(selector.LabelSelector, metav1.LabelSelector{}) && selector.MatchName == "" && selector.MatchStorageClassName == ""
}

func DownloadPodYaml(pod *corev1.Pod, saveFile string) error {
	if pod == nil {
		return nil
	}
	podYaml, err := yaml.Marshal(pod)
	if err != nil {
		return err
	}
	// Open a file to write logs
	file, err := os.Create(saveFile)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return err
	}
	defer file.Close()

	// Create a buffered writer
	writer := bufio.NewWriter(file)

	// Write the logs to the buffered writer
	_, err = io.Copy(writer, bytes.NewReader(podYaml))
	if err != nil {
		fmt.Println("Error in copying information from podYaml to file:", err)
		return err
	}

	// Flush the buffered writer to ensure all data is written to the file
	err = writer.Flush()
	if err != nil {
		fmt.Println("Error flushing writer:", err)
		return err
	}
	return nil
}

func ZipDir(source, target string) error {
	zipFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			relPath += "/"
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	return nil
}

func createTar(source, target string) error {
	tarFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	tarWriter := tar.NewWriter(tarFile)
	defer tarWriter.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create a relative path for the tar header
		relPath, err := filepath.Rel(filepath.Dir(source), path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil // Skip directories
		}

		// Create a tar header
		header, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write the header to the tarball
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Open the file to be added to the tarball
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Copy the file data to the tarball
		if _, err := io.Copy(tarWriter, file); err != nil {
			return err
		}

		return nil
	})
}
