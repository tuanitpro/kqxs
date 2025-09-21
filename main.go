package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

type RSS struct {
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

var rssURLs = map[string]string{
	"Miền Bắc":   "https://xosodaiphat.com/ket-qua-xo-so-mien-bac-xsmb.rss",
	"Miền Trung": "https://xosodaiphat.com/ket-qua-xo-so-mien-trung-xsmt.rss",
	"Miền Nam":   "https://xosodaiphat.com/ket-qua-xo-so-mien-nam-xsmn.rss",
}

var telegramBotToken string
var telegramChatID string

func init() {
	// Load .env
	err := godotenv.Load()
	if err != nil {
		fmt.Println("⚠️ Warning: .env file not found, using system environment variables")
	}

	telegramBotToken = os.Getenv("TELEGRAM_TOKEN")
	telegramChatID = os.Getenv("TELEGRAM_TO")

	if telegramBotToken == "" || telegramChatID == "" {
		fmt.Println("❌ TELEGRAM_TOKEN or TELEGRAM_TO is missing")
		os.Exit(1)
	}
}

func fetchRSS(url string) (*RSS, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rss RSS
	if err := xml.Unmarshal(data, &rss); err != nil {
		return nil, err
	}
	return &rss, nil
}

func parseDescription(desc string) map[string][]string {
	desc = strings.ReplaceAll(desc, "<br>", "\n")
	desc = strings.ReplaceAll(desc, "<br/>", "\n")
	desc = strings.ReplaceAll(desc, "<br />", "\n")

	lines := strings.Split(desc, "\n")

	results := make(map[string][]string)
	currentLocation := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Location
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			currentLocation = line
			continue
		}

		// Giải thưởng
		if strings.HasPrefix(line, "G.") {
			results[currentLocation] = append(results[currentLocation], line)
		}
	}

	return results
}

func sendToTelegram(message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", telegramBotToken)
	payload := fmt.Sprintf("chat_id=%s&text=%s", telegramChatID, message)

	resp, err := http.Post(url, "application/x-www-form-urlencoded", bytes.NewBufferString(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func runJob() {
	var finalMessage strings.Builder
	finalMessage.WriteString("🎰 *Kết quả xổ số hôm nay*\n\n")

	for region, url := range rssURLs {
		rss, err := fetchRSS(url)
		if err != nil {
			fmt.Println("Error fetching:", err)
			continue
		}
		if len(rss.Channel.Items) == 0 {
			fmt.Println("No items found for", region)
			continue
		}

		item := rss.Channel.Items[0]

		prizesByLocation := parseDescription(item.Description)

		fmt.Printf("=== %s | %s ===\n", region, item.Title)
		finalMessage.WriteString(fmt.Sprintf("📢 %s - %s\n", region, item.Title))

		for loc, prizes := range prizesByLocation {
			if loc != "" {
				fmt.Println(loc)
				finalMessage.WriteString(fmt.Sprintf("%s\n", loc))
			}
			for _, p := range prizes {
				fmt.Println(p)
				finalMessage.WriteString(fmt.Sprintf("%s\n", p))
			}
			fmt.Println()
			finalMessage.WriteString("\n")
		}
	}

	// Gửi message
	msg := finalMessage.String()
	if err := sendToTelegram(msg); err != nil {
		fmt.Println("Telegram error:", err)
	} else {
		fmt.Println("✅ Sent to Telegram successfully")
	}
}

func main() {
	runNow := flag.Bool("now", false, "Run the job immediately without waiting for schedule")
	flag.Parse()

	if *runNow {
		fmt.Println("🚀 Running job immediately (--now)")
		runJob()
		return
	}

	// Load timezone Vietnam
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		fmt.Println("❌ Cannot load timezone:", err)
		os.Exit(1)
	}

	c := cron.New(cron.WithLocation(loc))

	// Schedule hằng ngày 18h30
	_, err = c.AddFunc("30 18 * * *", runJob)
	if err != nil {
		fmt.Println("❌ Cannot schedule job:", err)
		os.Exit(1)
	}

	fmt.Println("⏰ Scheduler started... Waiting for 18:30 Asia/Ho_Chi_Minh")
	c.Start()

	// Giữ chương trình chạy
	select {}
}
