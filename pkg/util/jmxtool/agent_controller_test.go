/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
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
 */

package jmxtool

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"testing"
	"time"

	"github.com/megaease/easegress/pkg/filter/proxy"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

func httpServer(finished chan bool, notFoundFlag bool) {
	m := http.NewServeMux()
	s := http.Server{Addr: ":8181", Handler: m}
	m.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Goobey, %q", html.EscapeString(r.URL.Path))
		s.Shutdown(context.Background())
	})
	if !notFoundFlag {
		m.HandleFunc(serviceConfigURL, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
		})
		m.HandleFunc(canaryConfigURL, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
		})
	}
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Println(err)
	}
	fmt.Println("Finished")
	finished <- true
}

func getTestService() spec.Service {
	service := spec.Service{
		Name: "agent",
		LoadBalance: &spec.LoadBalance{
			Policy: proxy.PolicyRandom,
		},
		Sidecar: &spec.Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}
	return service
}

func TestAgentClientSuccess(t *testing.T) {
	logger.InitNop()
	finished := make(chan bool)
	go httpServer(finished, false)

	agent := NewAgentClient("127.0.0.1", "8181")
	fmt.Printf("%+v\n", agent)

	service := getTestService()
	// UpdateService check
	err := agent.UpdateService(&service, 1)
	if err != nil {
		t.Errorf("agent update service failed\n")
	}

	// UpdateCanary
	header := &spec.GlobalCanaryHeaders{
		ServiceHeaders: map[string][]string{},
	}
	err = agent.UpdateCanary(header, 1)
	if err != nil {
		t.Errorf("agent update canary failed\n")
	}

	// shutdown
	var client = &http.Client{
		Timeout: time.Second,
	}
	client.Get("http://127.0.0.1:8181/shutdown")
	<-finished
}

func TestAgentClientFail(t *testing.T) {
	logger.InitNop()
	agent := NewAgentClient("127.0.0.1", "8181")
	service := getTestService()

	// test without available service
	err := agent.UpdateService(&service, 1)
	if err == nil {
		t.Errorf("agent should fail\n")
	}
	header := &spec.GlobalCanaryHeaders{
		ServiceHeaders: map[string][]string{},
	}
	err = agent.UpdateCanary(header, 1)
	if err == nil {
		t.Errorf("agent should fail\n")
	}

	// test with 404
	finished := make(chan bool)
	go httpServer(finished, true)

	err = agent.UpdateService(&service, 1)
	if err == nil {
		t.Errorf("agent shoudl fail\n")
	}
	err = agent.UpdateCanary(header, 1)
	if err == nil {
		t.Errorf("agent shoudl fail\n")
	}
	// shutdown
	var client = &http.Client{
		Timeout: time.Second,
	}
	client.Get("http://127.0.0.1:8181/shutdown")
	<-finished
}