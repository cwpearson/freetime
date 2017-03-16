package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

	log "github.com/Sirupsen/logrus"
	tablewriter "github.com/olekukonko/tablewriter"
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

func contains(s string, slice []string) bool {
	for _, i := range slice {
		if i == s {
			return true
		}
	}
	return false
}

type Range struct {
	start time.Time
	end   time.Time
}

func (r *Range) Start() time.Time {
	return r.start
}

func (r *Range) Duration() time.Duration {
	return r.end.Sub(r.start)
}

func (r *Range) End() time.Time {
	return r.end
}

func (r *Range) String() string {
	fmt := "2 Mon 3:04"
	return r.Start().Format(fmt) + " - " + r.End().Format(fmt)
}

func (r *Range) split(start, end time.Time) []Range {

	// fmt.Println("Splitting ", r.String())
	// fmt.Println("  with ", (&Range{start, end}).String())

	ret := []Range{}

	// don't split if r starts after splitter ends
	if r.Start().After(end) || r.Start().Equal(end) {
		return []Range{*r}
	} else if r.End().Before(start) || r.End().Equal(start) { // don't split if splitter starts after r ends
		return []Range{*r}
	}

	// First range if Splitter starts during r
	if start.After(r.Start()) && start.Before(r.End()) {
		newRange := Range{r.Start(), start}
		// fmt.Println("  +", newRange.String())
		ret = append(ret, newRange)
	}

	// Second range if splitter ends during r
	if end.After(r.Start()) && end.Before(r.End()) {
		newRange := Range{end, r.End()}
		// fmt.Println("  +", newRange.String())
		ret = append(ret, newRange)
	}

	return ret
}

func (r *Range) After(t time.Time) Range {
	if t.After(r.End()) {
		return Range{r.End(), r.End()}
	} else if t.Before(r.Start()) {
		return *r
	} else {
		return Range{t, r.End()}
	}
}

// Gets the workday after or including now
func nextWorkDay(now time.Time) Range {
	var nextStart time.Time

	// if it's after 6pm, advance to tomorrow at 10am
	// otherwise, start at 10am today
	nowAtSix := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
	if now.After(nowAtSix) || now.Equal(nowAtSix) {
		nextStart = now.AddDate(0, 0, 1)                                                                               // tomorrow
		nextStart = time.Date(nextStart.Year(), nextStart.Month(), nextStart.Day(), 10, 0, 0, 0, nextStart.Location()) // 10am
	} else {
		nextStart = time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location()) // 10am
	}

	// If it's the weekend, advance to the next weekday
	if nextStart.Weekday() == time.Saturday {
		nextStart = nextStart.AddDate(0, 0, 2)
	} else if nextStart.Weekday() == time.Sunday {
		nextStart = nextStart.AddDate(0, 0, 1)
	}

	// End at 6pm the same day
	end := time.Date(nextStart.Year(), nextStart.Month(), nextStart.Day(), 18, 0, 0, 0, nextStart.Location())
	return Range{nextStart, end}
}

const (
	format = "2017-03-15T16:00:00-05:00"
)

func main() {
	ctx := context.Background()

	credPath := filepath.Join(os.Getenv("HOME"), ".credentials", "freetime_secret.json")
	b, err := ioutil.ReadFile(credPath)
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
	busySummaries := []string{"UIUC", "Social", "YMCA"}
	busyIDs := []string{}
	for _, i := range calendars.Items {
		if contains(i.Summary, busySummaries) {
			busyIDs = append(busyIDs, i.Id)
			// fmt.Println("Blocking with Calendar:", i.Summary)
		}

	}

	now := time.Now()
	busyEvents := []*calendar.Event{}
	freeRanges := []Range{}
	for i := 0; i < 3; i++ {
		nextDay := nextWorkDay(now.AddDate(0, 0, i))
		nextDay = nextDay.After(now) // don't look before now

		// fmt.Println(nextDay.String())

		freeRanges = append(freeRanges, nextDay)

		for _, busyID := range busyIDs {
			events, err := srv.Events.List(busyID).
				ShowDeleted(false).
				SingleEvents(true).
				TimeMin(nextDay.Start().Format(time.RFC3339)).
				TimeMax(nextDay.End().Format(time.RFC3339)).
				OrderBy("startTime").
				Do()

			if err != nil {
				log.Fatalf("Unable to retrieve user's events. %v", err)
			}

			for _, i := range events.Items {
				busyEvents = append(busyEvents, i)
				// fmt.Println("Upcoming: ", i.Start.DateTime, i.End.DateTime, i.Summary)
			}
			// fmt.Println("got ", len(busyEvents))
		}
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

	// Prune short ranges
	longRanges := []Range{}
	for _, r := range freeRanges {
		if r.Duration().Minutes() >= 30 {
			longRanges = append(longRanges, r)
		}
	}
	freeRanges = longRanges

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Day", "Start", "Stop"})

	for _, r := range freeRanges {
		table.Append([]string{r.Start().Format("Mon"), r.Start().Format("3:04"), r.End().Format("3:04")})
	}
	table.Render()
}
