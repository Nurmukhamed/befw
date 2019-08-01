/**
 * Copyright 2018-2019 Wargaming Group Limited
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
**/
package puppetdbsync

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var canRun = false
var canRunMutex = sync.RWMutex{}

var exitChan = make(chan bool, 10)

func Run(config string, timeout time.Duration) {
	syncConfig := newSync(config)
	syncConfig.timeout = timeout
	go makeCache(syncConfig)
	go keepLock(syncConfig)
	sigChan := make(chan os.Signal, 1)
	go func() {
		select {
		case <-sigChan:
			for i := 0; i < 10; i++ {
				exitChan <- true
			}
			syncConfig.cleanup()
		}
	}() // wait for signal
	signal.Notify(sigChan, syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM)
	for {
		canRunMutex.RLock()
		if canRun {
			for _, data := range syncConfig.requestPuppetDB() {
				syncConfig.servicesWG.Add(1)
				go syncConfig.writeSyncData(data)
			}
			syncConfig.servicesWG.Wait()
		}
		canRunMutex.RUnlock()
		select {
		case <-exitChan:
			return
		case <-time.After(syncConfig.timeout):
			continue
		}
	}
}

func makeCache(config *syncConfig) {
	for {
		config.makeHotCache()
		select {
		case <-exitChan:
			return
		case <-time.After(config.timeout):
			continue
		}
	}
}

func keepLock(config *syncConfig) {
	for {
		canRunMutex.Lock()
		canRun = config.manageSessionLock()
		canRunMutex.Unlock()
		select {
		case <-exitChan:
			return
		case <-time.After(config.timeout):
			continue
		}
	}
}
