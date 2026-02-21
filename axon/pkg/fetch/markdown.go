package fetch

import (
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
)

type Converter struct {
	conv *converter.Converter
}

func NewConverter() *Converter {
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)

	return &Converter{conv: conv}
}

func (c *Converter) Convert(html string) string {
	markdown, err := c.conv.ConvertString(html)
	if err != nil {
		return html
	}

	return markdown
}
