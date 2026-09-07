package gtm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/api/option"
	tagmanager "google.golang.org/api/tagmanager/v2"
)

// Inspect the actual generated API request: omitempty must not swallow clears.
func TestUpdateTriggerConditionPresence(t *testing.T) {
	for _, field := range []string{"filter", "autoEventFilter", "customEventFilter", "parameter"} {
		for _, mode := range []string{"omitted", "clear", "replace"} {
			t.Run(field+"/"+mode, func(t *testing.T) {
				var sent map[string]json.RawMessage
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`{"fingerprint":"original","filter":[{"type":"equals"}],"autoEventFilter":[{"type":"equals"}],"customEventFilter":[{"type":"equals"}],"parameter":[{"key":"old","type":"template","value":"old"}]}`))
						return
					}
					if r.Method != http.MethodPut || r.URL.Query().Get("fingerprint") != "original" {
						t.Errorf("unexpected update: %s %s", r.Method, r.URL)
					}
					if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
						t.Error(err)
					}
					_, _ = w.Write([]byte(`{"triggerId":"3"}`))
				}))
				defer ts.Close()
				service, err := tagmanager.NewService(context.Background(), option.WithEndpoint(ts.URL+"/"), option.WithoutAuthentication())
				if err != nil {
					t.Fatal(err)
				}
				input := &TriggerInput{Name: "test", Type: "customEvent"}
				if mode != "omitted" {
					conditions := []Condition{}
					params := []Parameter{}
					if mode == "replace" {
						conditions = []Condition{{Type: "contains"}}
						params = []Parameter{{Key: "new", Type: "template", Value: "new"}}
					}
					switch field {
					case "filter":
						input.Filter = conditions
					case "autoEventFilter":
						input.AutoEventFilter = conditions
					case "customEventFilter":
						input.CustomEventFilter = conditions
					case "parameter":
						input.Parameter = params
					}
				}
				if _, err := (&Client{Service: service}).UpdateTrigger(context.Background(), "accounts/1/containers/2/workspaces/2/triggers/3", input); err != nil {
					t.Fatal(err)
				}
				for _, key := range []string{"filter", "autoEventFilter", "customEventFilter", "parameter"} {
					var entries []map[string]any
					if err := json.Unmarshal(sent[key], &entries); err != nil {
						t.Fatalf("%s missing or invalid: %s", key, sent[key])
					}
					if key == field && mode == "clear" {
						if string(sent[key]) != "[]" {
							t.Errorf("clear %s sent %s, want []", key, sent[key])
						}
						continue
					}
					if len(entries) != 1 {
						t.Fatalf("%s lost its condition: %s", key, sent[key])
					}
					want := "equals"
					if mode == "replace" && key == field {
						want = "contains"
					}
					if key == "parameter" {
						want = "old"
						if mode == "replace" && key == field {
							want = "new"
						}
						if entries[0]["key"] != want {
							t.Errorf("parameter: %s", sent[key])
						}
					} else if entries[0]["type"] != want {
						t.Errorf("%s: %s", key, sent[key])
					}
				}
			})
		}
	}
}
