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

package device

import (
	"log"
	"os"
	"os/signal"

	"github.com/fsnotify/fsnotify"
)

func NewFSWatcher(files ...string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("new watcher meet error:%s\n", err.Error())
		return nil, err
	}

	for _, f := range files {
		err = watcher.Add(f)
		if err != nil {
			log.Printf("add watcher meet error:%s\n", err.Error())
			watcher.Close()
			return nil, err
		}
	}

	return watcher, nil
}

func NewOSWatcher(sigs ...os.Signal) chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sigs...)

	return sigChan
}
