package i18n

import (
	"reflect"
	"testing"

	"magpie-pulse/internal/model"
)

func TestResolveLocale(t *testing.T) {
	cases := []struct {
		cfgLang   model.Language
		ctxLocale string
		want      string
	}{
		{model.LangAuto, "", LocaleEn},
		{model.LangAuto, "en", LocaleEn},
		{model.LangAuto, "en-US", LocaleEn},
		{model.LangAuto, "fr-FR", LocaleEn},
		{model.LangAuto, "zh-Hans", LocaleZhHans},
		{model.LangAuto, "zh-Hans-CN", LocaleZhHans},
		{model.LangAuto, "zh-CN", LocaleZhHans},
		{model.LangAuto, "zh_CN", LocaleZhHans},
		{model.LangAuto, "zh-SG", LocaleZhHans},
		{model.LangAuto, "zh", LocaleZhHans},
		{model.LangAuto, "zh-Hant", LocaleEn},
		{model.LangZhHans, "", LocaleZhHans},
		{model.LangZhHans, "en-US", LocaleZhHans},
		{model.LangEn, "", LocaleEn},
		{model.LangEn, "zh-Hans", LocaleEn},
	}

	for _, tc := range cases {
		if got := Resolve(tc.cfgLang, tc.ctxLocale); got != tc.want {
			t.Errorf("Resolve(%q, %q) = %q, want %q", tc.cfgLang, tc.ctxLocale, got, tc.want)
		}
	}
}

func TestAllMessageFieldsPopulated(t *testing.T) {
	for _, msg := range []*Messages{&enMessages, &zhHansMessages} {
		v := reflect.ValueOf(msg).Elem()
		typ := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if f.Kind() == reflect.String && f.String() == "" {
				t.Errorf("locale %q has empty field %s", msg.Locale, typ.Field(i).Name)
			}
		}
	}
}

func TestMessageFormatHelpers(t *testing.T) {
	en := ByLocale(LocaleEn)
	if got := en.FormatConnected("0.5.0", 1); got != "Connected · v0.5.0 · 1 model" {
		t.Fatalf("en FormatConnected singular = %q", got)
	}
	if got := en.FormatConnected("v0.5.0", 12); got != "Connected · v0.5.0 · 12 models" {
		t.Fatalf("en FormatConnected plural = %q", got)
	}
	if got := en.FormatAccountsTotal(1); got != "1 account" {
		t.Fatalf("en FormatAccountsTotal(1) = %q", got)
	}
	if got := en.FormatAccountsVisible(2); got != "Showing 2 accounts in panel" {
		t.Fatalf("en FormatAccountsVisible(2) = %q", got)
	}
	if got := en.FormatTooltipAccounts(1); got != "Magpie Pulse · 1 account" {
		t.Fatalf("en FormatTooltipAccounts(1) = %q", got)
	}

	zh := ByLocale(LocaleZhHans)
	if got := zh.FormatConnected("0.5.0", 12); got != "已连接 · v0.5.0 · 12 个模型" {
		t.Fatalf("zh FormatConnected = %q", got)
	}
	if got := zh.FormatAccountsTotal(3); got != "共 3 个账号" {
		t.Fatalf("zh FormatAccountsTotal(3) = %q", got)
	}
	if got := zh.FormatAccountsVisible(2); got != "面板当前显示 2 个账号" {
		t.Fatalf("zh FormatAccountsVisible(2) = %q", got)
	}
	if got := zh.FormatTooltipAccounts(2); got != "Magpie Pulse · 2 个账号" {
		t.Fatalf("zh FormatTooltipAccounts(2) = %q", got)
	}
}
