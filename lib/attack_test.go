package stress

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAttackRate(t *testing.T) {
	t.Parallel()

	hitCount := uint64(0)
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddUint64(&hitCount, 1)
		}),
	)

	tgt := Target{Method: "GET", URL: server.URL}
	rate := uint64(5000)
	Attack(Targets{tgt}, rate, 1*time.Second)
	if hits := atomic.LoadUint64(&hitCount); hits != rate {
		t.Fatalf("Wrong number of hits: want %d, got %d\n", rate, hits)
	}
}

func TestAttackBody(t *testing.T) {
	t.Parallel()

	want := []byte("STRESS!")
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(want, got) {
				t.Fatalf("Wrong body. Want: %s, Got: %s", want, got)
			}
		}),
	)

	tgt := Target{Method: "GET", URL: server.URL, Body: want}
	rate := uint64(5000)
	Attack(Targets{tgt}, rate, 1*time.Second)
}

func TestDefaultAttackerCertConfig(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	request, _ := http.NewRequest("GET", server.URL, nil)
	_, err := DefaultAttacker.client.Do(request)
	if err != nil && strings.Contains(err.Error(), "x509: certificate signed by unknown authority") {
		t.Errorf("Invalid certificates should be ignored: Got `%s`", err)
	}
}

func TestSetRedirects(t *testing.T) {
	t.Parallel()

	var servers [2]*httptest.Server
	var hits uint64

	for i := range servers {
		servers[i] = httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddUint64(&hits, 1)
				http.Redirect(w, r, servers[(i+1)%2].URL, 302)
			}),
		)
	}

	DefaultAttacker.SetRedirects(2)

	tgt := Target{Method: "GET", URL: servers[0].URL}
	var rate uint64 = 100
	results := Attack(Targets{tgt}, rate, 1*time.Second)

	want := fmt.Sprintf("Stopped after %d redirects", 2)
	for _, result := range results {
		if !strings.Contains(result.Error, want) {
			t.Fatalf("Expected error to be: %s, Got: %s", want, result.Error)
		}
	}

	if want, got := rate*(2+1), hits; want != got {
		t.Fatalf("Expected hits to be: %d, Got: %d", want, got)
	}
}

func TestSetTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-time.After(2 * time.Second)
		}),
	)

	DefaultAttacker.SetTimeout(500 * time.Millisecond)

	tgt := Target{Method: "GET", URL: server.URL}
	results := Attack(Targets{tgt}, 100, 1*time.Second)

	want := "net/http: timeout awaiting response headers"
	for _, result := range results {
		if !strings.Contains(result.Error, want) {
			t.Fatalf("Expected error to be: %s, Got: %s", want, result.Error)
		}
	}
}
