// Package alert sends phone notifications through ntfy. Send never blocks or
// fails the request that triggered it: the push goes out on its own goroutine
// and a failure is only logged.
//
// The topic name is the only key: anyone who knows it can read these messages,
// so keep it long and random.
package alert

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	_ "time/tzdata" // the host image may ship no tz database; IST formatting needs it
)

var (
	topic  string
	client = &http.Client{Timeout: 10 * time.Second}
	ist    = loadIST()
)

func loadIST() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+30*60)
	}
	return loc
}

// Start enables alerts for the given ntfy topic. With it empty Send is a no-op.
func Start(ntfyTopic string) {
	topic = ntfyTopic
	if topic == "" {
		slog.Warn("NTFY_TOPIC is not set — phone alerts from the backend are off")
	}
}

// Send pushes one notification to the phone.
func Send(title, message string) {
	if topic == "" {
		return
	}
	go func() {
		req, err := http.NewRequest(http.MethodPost, "https://ntfy.sh/"+topic, strings.NewReader(message))
		if err != nil {
			slog.Error("alert: building request failed", "error", err)
			return
		}
		req.Header.Set("Title", title)
		req.Header.Set("Tags", "memo")
		resp, err := client.Do(req)
		if err != nil {
			slog.Error("alert: sending failed", "title", title, "error", err)
			return
		}
		resp.Body.Close()
	}()
}

// Punch announces an attendance event as "OFFICE IN" / "OFFICE OUT" with the
// employee's name and the event time in IST.
func Punch(direction, employeeName string, eventTime time.Time) {
	title, message := PunchMessage(direction, employeeName, eventTime)
	Send(title, message)
}

// PunchMessage builds the notification text for Punch.
func PunchMessage(direction, employeeName string, eventTime time.Time) (title, message string) {
	label := "Office " + map[string]string{"IN": "In", "OUT": "Out"}[direction]
	return "OFFICE " + direction, fmt.Sprintf("%s %s at %s", employeeName, label, eventTime.In(ist).Format("03:04 PM"))
}
