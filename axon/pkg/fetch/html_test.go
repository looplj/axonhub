package fetch

import "testing"

func TestExtractHTMLTitleAndDescription(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantTitle string
		wantDesc  string
	}{
		{
			name: "og_description_priority",
			html: `<html><head>
<title> Hello   World </title>
<meta property="description" content="plain desc">
<meta property="og:description" content="OG &amp; desc">
</head><body></body></html>`,
			wantTitle: "Hello World",
			wantDesc:  "OG & desc",
		},
		{
			name:      "property_description",
			html:      `<html><head><title>t</title><meta property="description" content="  a   b  "></head></html>`,
			wantTitle: "t",
			wantDesc:  "a b",
		},
		{
			name:      "name_description_fallback",
			html:      `<html><head><meta name="description" content="name desc"><title>t</title></head></html>`,
			wantTitle: "t",
			wantDesc:  "name desc",
		},
		{
			name:      "case_insensitive",
			html:      `<HTML><HEAD><TITLE> A&amp;B </TITLE><META PROPERTY="OG:DESCRIPTION" CONTENT=" X "></HEAD></HTML>`,
			wantTitle: "A&B",
			wantDesc:  "X",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotDesc := extractHTMLTitleAndDescription(tt.html)
			if gotTitle != tt.wantTitle {
				t.Fatalf("title: got %q want %q", gotTitle, tt.wantTitle)
			}

			if gotDesc != tt.wantDesc {
				t.Fatalf("description: got %q want %q", gotDesc, tt.wantDesc)
			}
		})
	}
}

func TestExtractTitleAndDescriptionDelegates(t *testing.T) {
	html := `<html><head><title>t</title><meta property="og:description" content="d"></head></html>`
	if got := extractTitle(html); got != "t" {
		t.Fatalf("extractTitle: got %q want %q", got, "t")
	}

	if got := extractDescription(html); got != "d" {
		t.Fatalf("extractDescription: got %q want %q", got, "d")
	}
}
