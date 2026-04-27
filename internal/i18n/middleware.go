// internal/i18n/middleware.go — Fiber middleware for language detection.
package i18n

import (
	"github.com/gofiber/fiber/v2"
)

const (
	LocaleKey   = "locale"    // c.Locals key for the active locale map
	LangKey     = "lang"      // c.Locals key for the active language code
	DirKey      = "dir"       // c.Locals key for text direction (ltr/rtl)
	CookieName  = "lang"      // browser cookie name
	DefaultLang = "en"
)

// RTL languages
var rtlLangs = map[string]bool{
	"ar": true,
	"he": true,
	"fa": true,
	"ur": true,
}

// Middleware returns a Fiber handler that resolves the active language and
// injects the locale map, language code, and text direction into c.Locals.
func Middleware(t Translations) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Query param takes precedence
		lang := c.Query("lang")

		// 2. Fall back to cookie
		if lang == "" {
			lang = c.Cookies(CookieName)
		}

		// 3. Default
		if lang == "" {
			lang = DefaultLang
		}

		// Ensure language exists, else default
		if _, ok := t[lang]; !ok {
			lang = DefaultLang
		}

		// Persist selection in cookie (1 year)
		if c.Query("lang") != "" {
			c.Cookie(&fiber.Cookie{
				Name:    CookieName,
				Value:   lang,
				MaxAge:  60 * 60 * 24 * 365,
				Path:    "/",
				SameSite: "Lax",
			})
		}

		dir := "ltr"
		if rtlLangs[lang] {
			dir = "rtl"
		}

		c.Locals(LocaleKey, t[lang])
		c.Locals(LangKey, lang)
		c.Locals(DirKey, dir)

		return c.Next()
	}
}

// GetLocale returns the active locale map from context.
func GetLocale(c *fiber.Ctx) map[string]string {
	if m, ok := c.Locals(LocaleKey).(map[string]string); ok {
		return m
	}
	return map[string]string{}
}

// GetLang returns the active language code from context.
func GetLang(c *fiber.Ctx) string {
	if s, ok := c.Locals(LangKey).(string); ok {
		return s
	}
	return DefaultLang
}

// GetDir returns the text direction for the active language.
func GetDir(c *fiber.Ctx) string {
	if s, ok := c.Locals(DirKey).(string); ok {
		return s
	}
	return "ltr"
}
