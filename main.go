package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	chromedp "github.com/chromedp/chromedp"
	"github.com/joho/godotenv"
	api2captcha "github.com/kpabellan/2captcha-go"
)

var waitGroup sync.WaitGroup

var captchaClientKey string = goDotEnvVariable("CAPTCHACLIENTKEY")

var catchall string = goDotEnvVariable("CATCHALL")

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

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

func generateKK() {
	defer waitGroup.Done()
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
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.102 Safari/537.36"),
	)

	cx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(cx)
	defer cancel()

	if err := chromedp.Run(ctx,
		chromedp.Navigate(kkUrl),
		chromedp.AttributeValue(`[data-sitekey]`, "data-sitekey", &siteKey, &siteKeyOk),
		chromedp.Click("#cookie-consent-container > div.cta > a.btn.btn-secondary.btn-ok.shadow"),  // Accept cookies
		chromedp.Evaluate(`document.querySelector("#btnSubmit").removeAttribute("disabled")`, nil), // Enable the submit button
		chromedp.SetAttributeValue("#btnSubmit", "class", "btn", chromedp.ByQuery),                 // Change submit button class to btn
		chromedp.SendKeys("#ctl00_plcMain_txtFirstName", "Krispy"),                                 // Input first name
		chromedp.SendKeys("#ctl00_plcMain_txtLastName", "Kreme"),                                   // Input last name
		chromedp.SendKeys("#ctl00_plcMain_ddlBirthdayMM", tomorrowMonth),                           // Input birthday month
		chromedp.SendKeys("#ctl00_plcMain_ddlBirthdayDD", tomorrowDay),                             // Input birthday day
		chromedp.SendKeys("#ctl00_plcMain_txtZipCode", "90001"),                                    // Input zipcode
		chromedp.SendKeys("#ctl00_plcMain_ucPhoneNumber_txt1st", randomCharNumerals(3)),            // Input phone number area code
		chromedp.SendKeys("#ctl00_plcMain_ucPhoneNumber_txt2nd", randomCharNumerals(3)),            // Input phone number first 3 digits
		chromedp.SendKeys("#ctl00_plcMain_ucPhoneNumber_txt3rd", randomCharNumerals(4)),            // Input phone number last 4 digits
		chromedp.SendKeys("#ctl00_plcMain_txtEmail", catchallEmail),                                // Input email
		chromedp.SendKeys("#ctl00_plcMain_txtPassword", password),                                  // Input password
		chromedp.SendKeys("#ctl00_plcMain_confirmPasswordTxt", password),                           // Confirm password
		chromedp.Click("#ctl00_plcMain_cbTermsOfUse"),                                              // Click terms of use
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
	); err != nil {
		fmt.Printf("Krispy Kreme Generation Failed. (%s)\n", err)
	} else {
		fmt.Printf("Krispy Kreme Generation Success!\n")
		fmt.Printf("Email: %s\nPassword: %s\n", catchallEmail, password)
	}
}

func main() {
	var genAmount int
	fmt.Print("Enter amount to generate: ")
	fmt.Scanf("%d", &genAmount)
	fmt.Printf("Generating %d Krispy Kreme accounts...\n", genAmount)

	for i := 0; i < genAmount; i++ {
		go generateKK()
		waitGroup.Add(1)
	}

	waitGroup.Wait()
}
