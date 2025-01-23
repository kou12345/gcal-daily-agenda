package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// カラーIDと色名のマッピング
var colorNames = map[string]string{
	"1":  "薄紫",
	"2":  "緑",
	"3":  "紫",
	"4":  "赤",
	"5":  "黄",
	"6":  "オレンジ",
	"7":  "水色",
	"8":  "グレー",
	"9":  "青紫",
	"10": "緑",
	"11": "赤",
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	// 日付引数の処理
	var targetDate time.Time
	var err error

	dateStr := flag.String("date", "", "Date to fetch events (format: YYYY-MM-DD)")
	flag.Parse()

	if *dateStr != "" {
		targetDate, err = time.Parse("2006-01-02", *dateStr)
		if err != nil {
			log.Fatalf("Invalid date format. Please use YYYY-MM-DD format: %v", err)
		}
	} else {
		targetDate = time.Now()
	}

	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	// 前日の開始時刻から当日の終了時刻までを設定
	startTime := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location()).
		AddDate(0, 0, -1) // 前日の00:00:00
	endTime := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 23, 59, 59, 0, targetDate.Location())

	events, err := srv.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(startTime.Format(time.RFC3339)).
		TimeMax(endTime.Format(time.RFC3339)).
		OrderBy("startTime").
		Do()
	if err != nil {
		log.Fatalf("Unable to retrieve events: %v", err)
	}

	// 日付を表示用にフォーマット
	displayDate := targetDate.Format("2006-01-02")
	fmt.Printf("%sの予定:\n", displayDate)

	if len(events.Items) == 0 {
		fmt.Printf("%sの予定はありません。\n", displayDate)
	} else {
		for _, item := range events.Items {
			// イベントの開始時刻と終了時刻を取得
			startDateTime := item.Start.DateTime
			if startDateTime == "" {
				startDateTime = item.Start.Date
			}
			endDateTime := item.End.DateTime
			if endDateTime == "" {
				endDateTime = item.End.Date
			}

			// イベントの開始時刻と終了時刻をパース
			eventStart, _ := time.Parse(time.RFC3339, startDateTime)
			eventEnd, _ := time.Parse(time.RFC3339, endDateTime)

			// イベントが指定された日に終了するか、指定された日をまたぐ場合に表示
			if (eventEnd.Format("2006-01-02") == displayDate) ||
				(eventStart.Before(endTime) && eventEnd.After(startTime.AddDate(0, 0, 1))) {

				// 表示形式を整える
				startDisplay := eventStart.Format("15:04")
				endDisplay := eventEnd.Format("15:04")

				// 色情報の取得と変換
				colorName := "デフォルト"
				if item.ColorId != "" {
					if name, ok := colorNames[item.ColorId]; ok {
						colorName = name
					}
				}

				// 終日イベントの場合は時刻を表示しない
				if item.Start.DateTime == "" {
					fmt.Printf("【%s】%v (終日) \n",
						colorName,
						item.Summary)
				} else {
					fmt.Printf("【%s】%v (%v-%v)\n",
						colorName,
						item.Summary,
						startDisplay,
						endDisplay)
				}
			}
		}
	}
}
