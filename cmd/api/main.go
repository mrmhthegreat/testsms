// cmd/api/main.go — Entry point for the SMS Gateway microservice.
package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"testsms/internal/i18n"
	"testsms/internal/sms"
	"testsms/pkg/queue"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// truncate shortens a UTF-8 string to at most n runes.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ─── template engine ─────────────────────────────────────────────────────────

// templateEngine renders HTML using Go's text/template.
// Fiber expects a custom engine; we use the lightweight built-in html approach
// via fiber's html template.

// ─── status badge ─────────────────────────────────────────────────────────

// statusBadgeHTML returns an HTMX-aware status badge.
// When status is not Delivered the badge includes polling attributes.
func statusBadgeHTML(id, status, translatedStatus string) template.HTML {
	var cls string
	switch status {
	case "Queued":
		cls = "badge badge-queued"
	case "Sending":
		cls = "badge badge-sending"
	case "Delivered":
		cls = "badge badge-delivered"
	default:
		cls = "badge"
	}

	htmxAttrs := ""
	if status != "Delivered" {
		htmxAttrs = fmt.Sprintf(
			` hx-get="/status/%s" hx-trigger="every 1s" hx-swap="outerHTML"`,
			id,
		)
	}

	return template.HTML(fmt.Sprintf(
		`<span id="status-%s" class="%s"%s>%s</span>`,
		id, cls, htmxAttrs, translatedStatus,
	))
}

// messageRow builds an HTML table row for a single message.
func messageRow(msg *sms.Message, statusBadge template.HTML) string {
	preview := truncate(msg.Body, 45)
	timeStr := msg.SentAt.Format("15:04:05")
	shortID := msg.ID[:8]
	encCls := "enc-gsm"
	if msg.Encoding != "GSM-7" {
		encCls = "enc-unicode"
	}
	return fmt.Sprintf(`
<tr id="row-%s">
  <td class="mono small">%s…</td>
  <td>%s</td>
  <td class="preview">%s</td>
  <td class="center">%d</td>
  <td><span class="%s">%s</span></td>
  <td>%s</td>
  <td class="mono small">%s</td>
</tr>`,
		msg.ID, shortID,
		template.HTMLEscapeString(msg.Phone),
		template.HTMLEscapeString(preview),
		msg.Segments,
		encCls, msg.Encoding,
		statusBadge,
		timeStr,
	)
}

// ─── logs buffer ─────────────────────────────────────────────────────────

type logBuffer struct {
	lines []string
	sync.Mutex
}

func (b *logBuffer) Write(p []byte) (n int, err error) {
	b.Lock()
	defer b.Unlock()
	b.lines = append(b.lines, string(p))
	if len(b.lines) > 20 {
		b.lines = b.lines[len(b.lines)-20:]
	}
	return len(p), nil
}

var globalLogs logBuffer

// ─── main ────────────────────────────────────────────────────────────────────

func main() {
	redisAddr := env("REDIS_ADDR", "localhost:6379")
	port := env("PORT", "3000")

	// ── Redis & services ──────────────────────────────────────────────────
	log.SetOutput(io.MultiWriter(os.Stdout, &globalLogs))
	q := queue.New(redisAddr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := q.Ping(ctx); err != nil {
		log.Fatalf("[main] redis connection failed: %v", err)
	}
	log.Printf("[main] connected to Redis at %s", redisAddr)

	repo := sms.NewRepository(q)
	svc := sms.NewService(repo)
	worker := sms.NewWorker(repo)

	// ── Background worker ────────────────────────────────────────────────
	go worker.Run(ctx)

	// ── i18n ─────────────────────────────────────────────────────────────
	translations, err := i18n.LoadLocales("locales")
	if err != nil {
		log.Fatalf("[main] failed to load locales: %v", err)
	}

	// ── Fiber app ─────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:      "SMS Gateway v1.0",
		ErrorHandler: errorHandler,
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} ${method} ${path} ${latency}\n",
	}))
	app.Use(i18n.Middleware(translations))

	// ── Routes ───────────────────────────────────────────────────────────
	app.Get("/", func(c *fiber.Ctx) error {
		locale := i18n.GetLocale(c)
		lang := i18n.GetLang(c)
		dir := i18n.GetDir(c)

		html, err := os.ReadFile("views/index.html")
		if err != nil {
			return fiber.ErrInternalServerError
		}

		toggleLang := "ar"
		if lang == "ar" {
			toggleLang = "en"
		}

		// Inject server-side values into the template
		page := string(html)
		page = strings.ReplaceAll(page, "{{.Lang}}", lang)
		page = strings.ReplaceAll(page, "{{.Dir}}", dir)
		page = strings.ReplaceAll(page, "{{.ToggleLang}}", toggleLang)

		// Replace all {{t "key"}} patterns with translated values
		for key, val := range locale {
			page = strings.ReplaceAll(page, fmt.Sprintf(`{{t "%s"}}`, key), val)
		}

		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(page)
	})

	app.Post("/send-sms", func(c *fiber.Ctx) error {
		locale := i18n.GetLocale(c)

		phone := strings.TrimSpace(c.FormValue("phone"))
		body := strings.TrimSpace(c.FormValue("message"))

		msg, err := svc.Send(ctx, phone, body)
		if err != nil {
			return c.Status(400).SendString(fmt.Sprintf(
				`<tr><td colspan="7" style="color:var(--warning);text-align:center;">%s</td></tr>`,
				locale["alert_error"],
			))
		}

		badge := statusBadgeHTML(msg.ID, msg.Status, locale["status_queued"])
		row := messageRow(msg, badge)
		
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(row)
	})

	app.Post("/send-bulk", func(c *fiber.Ctx) error {
		locale := i18n.GetLocale(c)

		rawPhones := strings.TrimSpace(c.FormValue("phones"))
		body := strings.TrimSpace(c.FormValue("message"))

		if rawPhones == "" || body == "" {
			return c.Status(400).SendString("")
		}

		// Split phones by newline, comma, or space
		phoneLines := strings.FieldsFunc(rawPhones, func(r rune) bool {
			return r == '\n' || r == ',' || r == ' ' || r == '\r'
		})

		var html strings.Builder
		for _, phone := range phoneLines {
			phone = strings.TrimSpace(phone)
			if phone == "" {
				continue
			}

			msg, err := svc.Send(ctx, phone, body)
			if err != nil {
				continue // Skip failing individual numbers quietly in bulk send
			}

			badge := statusBadgeHTML(msg.ID, msg.Status, locale["status_queued"])
			html.WriteString(messageRow(msg, badge))
		}

		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(html.String())
	})

	app.Get("/status/:id", func(c *fiber.Ctx) error {
		locale := i18n.GetLocale(c)
		id := c.Params("id")

		status, err := svc.GetStatus(ctx, id)
		if err != nil {
			status = "Queued"
		}

		var translatedStatus string
		switch status {
		case "Queued":
			translatedStatus = locale["status_queued"]
		case "Sending":
			translatedStatus = locale["status_sending"]
		case "Delivered":
			translatedStatus = locale["status_delivered"]
		default:
			translatedStatus = status
		}

		badge := statusBadgeHTML(id, status, translatedStatus)
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(string(badge))
	})

	app.Get("/redis-logs", func(c *fiber.Ctx) error {
		globalLogs.Lock()
		defer globalLogs.Unlock()
		return c.SendString(strings.Join(globalLogs.lines, ""))
	})

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("[main] shutting down…")
		cancel()
		_ = app.ShutdownWithTimeout(5 * time.Second)
	}()

	log.Printf("[main] SMS Gateway listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("[main] server error: %v", err)
	}
}

// ── Error handler ────────────────────────────────────────────────────────────

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(code).SendString(fmt.Sprintf(
		`<div class="text-red-400 p-4">Error %d: %s</div>`, code, err.Error(),
	))
}
