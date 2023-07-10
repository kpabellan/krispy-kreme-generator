package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	chromedp "github.com/chromedp/chromedp"
	"github.com/joho/godotenv"
	api2captcha "github.com/kpabellan/2captcha-go"
)

var waitGroup sync.WaitGroup

var captchaClientKey string = goDotEnvVariable("CAPTCHACLIENTKEY")

var catchall string = goDotEnvVariable("CATCHALL")

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

var userAgents = []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/601.7.7 (KHTML, like Gecko) Version/9.1.2 Safari/601.7.7", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36", "Mozilla/5.0 (Windows NT 10.0; WOW64; rv:56.0) Gecko/20100101 Firefox/56.0", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.10240", "Mozilla/5.0 (iPad; CPU OS 10_2_1 like Mac OS X) AppleWebKit/602.4.6 (KHTML, like Gecko) Version/10.0 Mobile/14D27 Safari/602.1)", "Mozilla/5.0 (Linux; Android 7.1; vivo 1716 Build/N2G47H) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.98 Mobile Safari/537.36"}

func goDotEnvVariable(key string) string {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

func randomCharNumerals(length int) string {
	const charset = "0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func formatProxy(proxy string) string {
	parts := strings.Split(proxy, ":")
	if len(parts) != 4 {
		if len(parts) == 2 {
			ip := parts[0]
			port := parts[1]
			return fmt.Sprintf("http://%s:%s", ip, port)
		}
		fmt.Println("Invalid proxy format:", proxy)
		return ""
	}

	ip := parts[0]
	port := parts[1]
	username := parts[2]
	password := parts[3]

	return fmt.Sprintf("http://%s:%s@%s:%s", username, password, ip, port)
}

func runFunc(timeout time.Duration, task chromedp.ActionFunc) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return task.Do(ctx)
	}
}

func runTask(timeout time.Duration, tasks chromedp.Tasks) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		timeoutContext, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return tasks.Do(timeoutContext)
	}
}

func solveReCaptcha(client *api2captcha.Client, kkUrl, dataSiteKey string) (string, error) {
	c := api2captcha.ReCaptcha{
		SiteKey:   dataSiteKey,
		Url:       kkUrl,
		Invisible: true,
		Action:    "verify",
	}

	return client.Solve(c.ToRequest())
}

func generateKK(proxy string) {
	defer waitGroup.Done()

	os.Setenv("HTTP_PROXY", proxy)

	client := api2captcha.NewClient(captchaClientKey)
	kkUrl := "https://www.krispykreme.com/account/create-account"
	tomorrow := time.Now().AddDate(0, 0, 1)
	tomorrowDay := strconv.Itoa(tomorrow.Day())
	tomorrowMonth := strconv.Itoa(int(tomorrow.Month()))
	catchallEmail := "kp" + randomCharNumerals(5) + "@" + catchall
	password := "Donuttime123"
	var siteKey string
	var siteKeyOk bool

	if len(tomorrowDay) == 1 {
		tomorrowDay = "0" + tomorrowDay
	}

	if len(tomorrowMonth) == 1 {
		tomorrowMonth = "0" + tomorrowMonth
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(userAgents[seededRand.Intn(len(userAgents))]),
	)

	cx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(cx)
	defer cancel()

	delay := func(min, max time.Duration) {
		time.Sleep(min + time.Duration(seededRand.Int63n(int64(max-min))))
	}

	typeAction := func(selector, text string) chromedp.ActionFunc {
		return func(ctx context.Context) error {
			for _, r := range text {
				chromedp.SendKeys(selector, string(r)).Do(ctx)
				delay(50*time.Millisecond, 200*time.Millisecond)
			}
			return nil
		}
	}

	var tasks []chromedp.Action
	tasks = append(tasks,
		chromedp.Navigate(kkUrl),
		chromedp.AttributeValue(`[data-sitekey]`, "data-sitekey", &siteKey, &siteKeyOk),
		chromedp.Click("#ctl00_plcMain_cbTermsOfUse"),                                                 // Click terms of use
		chromedp.Evaluate(`document.querySelector("#btnSubmit").removeAttribute("disabled")`, nil),    // Enable the submit button
		chromedp.SetAttributeValue("#btnSubmit", "class", "btn", chromedp.ByQuery),                    // Change submit button class to btn
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_txtFirstName", "Krispy")),                      // Input first name
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_txtLastName", "Kreme")),                        // Input last name
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_ddlBirthdayMM", tomorrowMonth)),                // Input birthday month
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_ddlBirthdayDD", tomorrowDay)),                  // Input birthday day
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_txtZipCode", "90001")),                         // Input zipcode
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_ucPhoneNumber_txt1st", randomCharNumerals(3))), // Input phone number area code
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_ucPhoneNumber_txt2nd", randomCharNumerals(3))), // Input phone number first 3 digits
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_ucPhoneNumber_txt3rd", randomCharNumerals(4))), // Input phone number last 4 digits
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_txtEmail", catchallEmail)),                     // Input email
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_txtPassword", password)),                       // Input password
		chromedp.ActionFunc(typeAction("#ctl00_plcMain_confirmPasswordTxt", password)),                // Input confirm password

		runFunc(5*time.Minute, func(ctx context.Context) error {
			if !siteKeyOk {
				return errors.New("missing data-sitekey")
			}

			token, err := solveReCaptcha(client, kkUrl, siteKey)
			if err != nil {
				return err
			}

			return chromedp.SetJavascriptAttribute(`#g-recaptcha-response`, "innerText", token).Do(ctx)
		}),
		chromedp.Click("#btnSubmit", chromedp.ByQuery), // Submit form
		runTask(3*time.Minute, chromedp.Tasks{
			chromedp.WaitVisible(`a[href*="twitter"]`, chromedp.ByQuery),
		}),
	)

	if err := chromedp.Run(ctx, tasks...); err != nil {
		fmt.Printf("Krispy Kreme Generation Failed. (%s)\n", err)
	} else {
		fmt.Printf("Krispy Kreme Generation Success!\n")
		fmt.Printf("Email: %s\nPassword: %s\n", catchallEmail, password)
	}
}

func main() {
	lines, err := readLines("proxylist.txt")
	if err != nil {
		log.Fatalf("readLines: %s", err)
	}

	var genAmount int
	fmt.Print("Enter amount to generate: ")
	fmt.Scanf("%d", &genAmount)
	fmt.Printf("Generating %d Krispy Kreme accounts...\n", genAmount)

	for i := 0; i < genAmount; i++ {
		if i != 0 && i%3 == 0 {
			time.Sleep(150 * time.Second)
		}

		proxy := lines[seededRand.Intn(len(lines))]
		formattedProxy := formatProxy(proxy)

		go generateKK(formattedProxy)
		time.Sleep(1 * time.Second)
		waitGroup.Add(1)
	}

	waitGroup.Wait()
}
