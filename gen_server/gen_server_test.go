// Copyright (c) 2013 Couchbase, Inc.
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
// except in compliance with the License. You may obtain a copy of the License at
//   http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the
// License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing permissions
// and limitations under the License.

package gen_server

import (
	"errors"
	"fmt"
	common "github.com/Xiaomei-Zhang/goxdcr/common"
	part "github.com/Xiaomei-Zhang/goxdcr/part"
	base "github.com/Xiaomei-Zhang/couchbase_goxdcr_impl/base"
	"sync"
	"testing"
	"time"
)

type example_part struct {
	dataChan chan interface{}
	GenServer
	part.AbstractPart
	increase_amount int
	waitGrp         sync.WaitGroup
}

func newExamplePart(id string, increase_amount int) *example_part {
	var exit_callback gen_server.Exit_Callback_Func
	var extend_callback gen_server.Behavior_Callback_Func
	part_server := gen_server.NewGenServer(nil, &extend_callback, &exit_callback)
	part := &example_part{make(chan interface{}), part_server, part.NewAbstractPart(id), increase_amount, sync.WaitGroup{}}
	exit_callback = part.onExit
	extend_callback = part.runData
	return part
}
func (p *example_part) runData() {
	select {
	case data := <-p.dataChan:
		go p.processData(data)
		p.waitGrp.Add(1)
	default:

	}
}

func (p *example_part) Start(settings map[string]interface{}) error {
	return p.Start_server()
}

func (p *example_part) Stop() error {
	return p.Stop_server()
}

func (p *example_part) processData(data interface{}) {
	newData := data.(int) + p.increase_amount
	fmt.Println(fmt.Sprint(newData))
	//raise DataProcessed event
	p.RaiseEvent(common.DataProcessed, data, p, nil, nil)

	p.Connector().Forward(newData)

	p.waitGrp.Done()

}

func (p *example_part) onExit() {
	fmt.Println("Before Exit")
	p.waitGrp.Wait()
}

func (p *example_part) Receive(data interface{}) error {
	if p.dataChan == nil || !p.IsStarted() {
		return errors.New("The Part is not running, not ready to process data")
	}
	p.dataChan <- data

	//raise DataReceived event
	p.RaiseEvent(common.DataReceived, data, p, nil, nil)

	return nil
}

func TestStartStop(t *testing.T) {
	p := newExamplePart("XYZ", 10)
	t.Log("Part XYZ is created")
	err := p.Start(make(map[string]interface{}))

	if err == nil {
		time.Sleep(2 * time.Second)
		p.Stop()
	}
}

func TestHeartBeat(t *testing.T) {
	p := newExamplePart("XYZ", 12)
	t.Log("Part XYZ is created")
	err := p.Start(make(map[string]interface{}))

	if err == nil {
		var waitGrp sync.WaitGroup
		finchan := make(chan bool)
		finchan2 := make(chan bool)
		ticker := time.NewTicker(200 * time.Millisecond)
		//launch the heartbeat checker
		waitGrp.Add(1)
		go heartBeatChecker(p, ticker.C, finchan2, &waitGrp)

		//launch life killer
		waitGrp.Add(1)
		go lifeKiller(p, finchan, 5*time.Second, t, &waitGrp)

	loop2:
		for {
			select {
			case <-finchan:
				finchan2 <- true
				ticker.Stop()

				break loop2
			default:
				//				fmt.Printf("finchan %s = %d\n", fmt.Sprint(finchan), len(finchan))
			}
		}

		waitGrp.Wait()
	}
}

func heartBeatChecker(p *example_part, timechan <-chan time.Time, finchan chan bool, waitGrp *sync.WaitGroup) {
	defer waitGrp.Done()
loop:
	for {
		for now := range timechan {
			select {
			case <-finchan:
				break loop
			default:
				if !p.HeartBeat() {
					//yell
					fmt.Printf("Part %s is dead\n", p.Id())
				} else {
					fmt.Printf("%s - All well\n", fmt.Sprint(now))
				}

			}
		}
	}

}

func lifeKiller(p *example_part, finchan chan bool, lifespan time.Duration, t *testing.T, waitGrp *sync.WaitGroup) {
	defer waitGrp.Done()
	time.Sleep(lifespan)

	fmt.Println("Wakeup to kill")

	err := p.Stop()
	if err != nil {
		t.Errorf("Failed to stop %s", p.Id())
	}
	finchan <- true
	fmt.Println("Killer is done")
}
