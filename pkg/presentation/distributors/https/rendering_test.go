package https

import (
	"io"
	"testing"

	"golang.org/x/text/language"
)

func Test_getPrimaryLanguage(t *testing.T) {
	type args struct {
		requestedLanguages []string
		supportedLanguages []language.Tag
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test 1",
			args: args{
				requestedLanguages: []string{"en", "es"},
				supportedLanguages: []language.Tag{language.English, language.Spanish},
			},
			want: "en",
		},
		{
			name: "Test 2",
			args: args{
				requestedLanguages: []string{"es", "en"},
				supportedLanguages: []language.Tag{language.English, language.Spanish},
			},
			want: "es",
		},
		{
			name: "Test 3",
			args: args{
				requestedLanguages: []string{"es", "en   "},
				supportedLanguages: []language.Tag{language.English},
			},
			want: "en",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPrimaryLanguage(tt.args.requestedLanguages, tt.args.supportedLanguages); got != tt.want {
				t.Errorf("getPrimaryLanguage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRendering(t *testing.T) {
	r, err := newRenderingContext()
	if err != nil {
		t.Error(err)
	}
	err = r.render("homepage.html", nil, io.Discard)
	if err != nil {
		t.Error(err)
	}
}
