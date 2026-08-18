package compatcheck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// Oracle is one SMTP ingest + management HTTP listener.
type Oracle struct {
	Name string
	SMTP string // host:port
	HTTP string // host:port
	User string
	Pass string
}

// Report is one probe of one oracle with one RFC 5321 message.
type Report struct {
	Name           string
	UnauthStatus   int
	HealthzStatus  int
	ListStatus     int
	ListIsArray    bool
	ListSubjectHit bool
	ListItem       map[string]any
	GetStatus      int
	GetItem        map[string]any
	RelayStatus    int
	Subject        string
}

// Probe sends raw via net/smtp.SendMail (no AUTH) and applies the swap-gate
// HTTP checks to the management listener.
func Probe(o Oracle, raw []byte, from string, to []string, subject string) (Report, error) {
	r := Report{Name: o.Name, Subject: subject}
	if err := smtp.SendMail(o.SMTP, nil, from, to, raw); err != nil {
		return r, fmt.Errorf("%s SendMail: %w", o.Name, err)
	}

	unauth, body, err := doHTTP(http.MethodGet, "http://"+o.HTTP+"/email", "", "")
	if err != nil {
		return r, fmt.Errorf("%s GET /email unauth: %w", o.Name, err)
	}
	r.UnauthStatus = unauth
	_ = body

	hz, _, err := doHTTP(http.MethodGet, "http://"+o.HTTP+"/healthz", "", "")
	if err != nil {
		return r, fmt.Errorf("%s GET /healthz: %w", o.Name, err)
	}
	r.HealthzStatus = hz

	listStatus, listBody, err := doHTTP(http.MethodGet, "http://"+o.HTTP+"/email", o.User, o.Pass)
	if err != nil {
		return r, fmt.Errorf("%s GET /email basic: %w", o.Name, err)
	}
	r.ListStatus = listStatus
	var items []map[string]any
	if err := json.Unmarshal(listBody, &items); err == nil {
		r.ListIsArray = true
		for _, it := range items {
			if str(it["subject"]) == subject {
				r.ListSubjectHit = true
				r.ListItem = it
				break
			}
		}
	}

	if id := str(r.ListItem["id"]); id != "" {
		gs, gb, err := doHTTP(http.MethodGet, "http://"+o.HTTP+"/email/"+id, o.User, o.Pass)
		if err != nil {
			return r, fmt.Errorf("%s GET /email/%s: %w", o.Name, id, err)
		}
		r.GetStatus = gs
		_ = json.Unmarshal(gb, &r.GetItem)

		rs, _, err := doHTTP(http.MethodPost, "http://"+o.HTTP+"/email/"+id+"/relay", o.User, o.Pass)
		if err != nil {
			return r, fmt.Errorf("%s POST relay: %w", o.Name, err)
		}
		r.RelayStatus = rs
	}
	return r, nil
}

func doHTTP(method, url, user, pass string) (int, []byte, error) {
	req, err := http.NewRequest(method, url, strings.NewReader("{}"))
	if err != nil {
		return 0, nil, err
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// FormatTranscript is a one-line swap-gate record for logs.
func FormatTranscript(r Report) string {
	id := ""
	if r.ListItem != nil {
		id = str(r.ListItem["id"])
	}
	listText := ""
	if r.ListItem != nil {
		listText = str(r.ListItem["text"])
	}
	return fmt.Sprintf("%s smtp+http probe subject=%q unauth=%d healthz=%d list=%d array=%v subject_hit=%v id=%s list_text_empty=%v get=%d relay=%d",
		r.Name, r.Subject, r.UnauthStatus, r.HealthzStatus, r.ListStatus, r.ListIsArray, r.ListSubjectHit, id, listText == "", r.GetStatus, r.RelayStatus)
}
