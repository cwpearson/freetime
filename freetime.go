package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
	}
	return config.Client(ctx, tok)
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// tokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename.
func tokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir,
		url.QueryEscape("calendar-go-quickstart.json")), err
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.Create(file)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func nextWorkDay(now time.Time) (start, end time.Time) {
	var next time.Time

	if now.Weekday() == time.Saturday {
		next = now.AddDate(0, 0, 2)
	} else if now.Weekday() == time.Sunday {
		next = now.AddDate(0, 0, 1)
	} else {
		next = now
	}

	// 10am or now, whenever is later
	start = time.Date(next.Year(), next.Month(), next.Day(), 10, 0, 0, 0, now.Location())
	if start.After(now) {
		start = now
	}

	// 6pm
	end = time.Date(next.Year(), next.Month(), next.Day(), 18, 0, 0, 0, now.Location())
	return start, end
}

func contains(s string, slice []string) bool {
	for _, i := range slice {
		if i == s {
			return true
		}
	}
	return false
}

type Range struct {
	start    time.Time
	duration time.Duration
}

func (r *Range) Start() time.Time {
	return r.Start()
}

func (r *Range) Duration() time.Duration {
	return r.duration
}

func (r *Range) End() time.Time {
	return r.Start().Add(r.Duration())
}

func (r *Range) split(start, end time.Time) []Range {

	// don't split if range starts after splitter ends
	if r.Start().After(end) {
		return []Range{*r}
	} else if start.After(r.End()) { // don't split if splitter starts after range ends
		return []Range{*r}
	}

	return []Range{}
}

const (
	format = "2017-03-15T16:00:00-05:00"
)

func main() {
	ctx := context.Background()

	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved credentials
	// at ~/.credentials/calendar-go-quickstart.json
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(ctx, config)

	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve calendar Client %v", err)
	}

	calendars, err := srv.CalendarList.List().Do()
	if err != nil {
		log.Fatalf("Unable to list calendars. %v", err)
	}

	// find IDs of all calendars that cause me to be busy
	busySummaries := []string{"UIUC", "Personal", "YMCA"}
	busyIDs := []string{}
	for _, i := range calendars.Items {
		if contains(i.Summary, busySummaries) {
			busyIDs = append(busyIDs, i.Id)
			fmt.Println("Blocking with Calendar: ", i.Summary)
		}

	}

	now := time.Now()
	busyEvents := []*calendar.Event{}
	freeRanges := []Range{}
	for i := 0; i < 3; i++ {
		dayStart, dayStop := nextWorkDay(now.AddDate(0, 0, i))
		freeRanges = append(freeRanges, Range{dayStart, dayStop.Sub(dayStart)})

		for _, busyID := range busyIDs {
			events, err := srv.Events.List(busyID).
				ShowDeleted(false).
				SingleEvents(true).
				TimeMin(dayStart.Format(time.RFC3339)).
				TimeMax(dayStop.Format(time.RFC3339)).
				OrderBy("startTime").
				Do()

			if err != nil {
				log.Fatalf("Unable to retrieve user's events. %v", err)
			}

			for _, i := range events.Items {
				busyEvents = append(busyEvents, i)
				fmt.Println("Blocked by", i.Summary)
			}
		}

		// fmt.Println("Upcoming events:")
		// if len(events.Items) > 0 {
		// 	for _, i := range events.Items {
		// 		var when string
		// 		// If the DateTime is an empty string the Event is an all-day Event.
		// 		// So only Date is available.
		// 		if i.Start.DateTime != "" {
		// 			when = i.Start.DateTime
		// 		} else {
		// 			when = i.Start.Date
		// 		}
		// 		fmt.Printf("%s (%s)\n", i.Summary, when)
		// 	}
		// } else {
		// 	fmt.Printf("No upcoming events found.\n")
		// }
	}

	for _, event := range busyEvents {
		if event.Start.DateTime != "" {
			newRanges := []Range{}
			for _, r := range freeRanges {
				eventStart, err := time.Parse(time.RFC3339, event.Start.DateTime)
				if err != nil {
					log.Fatalf("Unable to parse time. %v", err)
				}
				eventEnd, err := time.Parse(time.RFC3339, event.End.DateTime)
				if err != nil {
					log.Fatalf("Unable to parse time. %v", err)
				}
				splitRange := r.split(eventStart, eventEnd)
				for _, split := range splitRange {
					newRanges = append(newRanges, split)
				}
			}

			freeRanges = newRanges
		}
	}

	for _, r := range freeRanges {
		fmt.Println(r)
	}

}
