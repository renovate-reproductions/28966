package https

import (
	"embed"
	"html/template"
	"io"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/locales"
)

//go:embed embedded
var embedfs embed.FS

type renderingContext struct {
	templates *template.Template
	localizer *i18n.Localizer
	Input     map[string]interface{}

	LocalizerPrimaryLanguage string
	LocalizerTextDirection   string
}

func (r *renderingContext) render(path string, input map[string]interface{}, wr io.Writer) error {
	r.Input = input
	if r.Input == nil {
		r.Input = make(map[string]interface{})
	}
	return r.templates.ExecuteTemplate(wr, path, r)
}

func (r *renderingContext) swallowCopy() *renderingContext {
	return &renderingContext{
		templates:                r.templates,
		localizer:                r.localizer,
		Input:                    r.Input,
		LocalizerPrimaryLanguage: r.LocalizerPrimaryLanguage,
		LocalizerTextDirection:   r.LocalizerTextDirection,
	}
}

func (r *renderingContext) createFuncMap() template.FuncMap {
	return map[string]any{
		"translated": func(input string) string {
			result, err := r.localizer.Localize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    input,
					Other: input,
				},
			})
			if err != nil {
				return input
			}
			return result
		},
		"constructMapContext": func(arg ...string) *renderingContext {
			var input = make(map[string]interface{})
			for i := 0; i < len(arg); i += 2 {
				input[arg[i]] = arg[i+1]
			}
			copied := r.swallowCopy()
			copied.Input = input
			return copied
		},
		"asseturl": func(input string) string {
			return input
		},
		"url": func(input string) string {
			return input
		},
		"setMapContext": func(arg ...string) string {
			for i := 0; i < len(arg); i += 2 {
				r.Input[arg[i]] = arg[i+1]
			}
			return ""
		},
	}
}

func newRenderingContextWithOpts(langs []string) (*renderingContext, error) {
	rContext := &renderingContext{}

	translationBundle, err := locales.NewBundle()
	if err != nil {
		return nil, err
	}
	rContext.localizer = i18n.NewLocalizer(translationBundle, langs...)
	rContext.templates = template.New("root").Funcs(rContext.createFuncMap())
	_, err = rContext.templates.ParseFS(embedfs, "embedded/*.html")
	if err != nil {
		return nil, err
	}
	rContext.LocalizerPrimaryLanguage = getPrimaryLanguage(langs, translationBundle.LanguageTags())
	rContext.LocalizerTextDirection = getLanguageDirection(rContext.LocalizerPrimaryLanguage)
	return rContext, nil
}

func newRenderingContext() (*renderingContext, error) {
	return newRenderingContextWithOpts([]string{"en"})
}

func getLanguageDirection(lang string) string {
	// !!! Some languages have more than one writing systems,
	//     with different directions. This function is an oversimplification.
	//     Top to bottom languages support is currently absent.
	switch lang {
	// TODO: list all rtl languages
	case "ar", "fa", "he", "ur", "dv", "ps", "ku", "sd", "ug", "yi":
		return "rtl"
	default:
		return "ltr"
	}
}

func getPrimaryLanguage(requestedLanguages []string, supportedLanguages []language.Tag) string {
	tags := parseTags(requestedLanguages)
	if len(tags) == 0 {
		return "en"
	}
	var primaryLanguage = "en"
	matchedLanguage, _, _ := language.NewMatcher(supportedLanguages).Match(tags...)
	languageBase, _ := matchedLanguage.Base()
	if languageBase.String() != "" {
		primaryLanguage = languageBase.String()
	}
	return primaryLanguage
}

func parseTags(langs []string) []language.Tag {
	tags := []language.Tag{}
	for _, lang := range langs {
		t, _, err := language.ParseAcceptLanguage(lang)
		if err != nil {
			continue
		}
		tags = append(tags, t...)
	}
	return tags
}
