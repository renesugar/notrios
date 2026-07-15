package markdownlinks

import "testing"

func TestExtractMarkdownAndWikiLinks(t *testing.T) {
	body := "# A\nSee [B](document://default/documents/doc_b#Heading).\n![Img](resource://default/resources/res_1)\nAlso [[Note C|alias]] and ![[Other#^block]]."
	links := Extract(body)
	if len(links) != 4 {
		t.Fatalf("got %d links, want 4: %#v", len(links), links)
	}
	if links[0].RawTarget != "document://default/documents/doc_b" || links[0].AnchorType != "heading" || links[0].AnchorValue != "Heading" {
		t.Fatalf("unexpected first link: %#v", links[0])
	}
	if links[1].RelationType != "embed" || links[1].RawTarget != "resource://default/resources/res_1" {
		t.Fatalf("unexpected image link: %#v", links[1])
	}
	if links[2].SourceFormat != "obsidian-wikilink" || links[2].RawTarget != "Note C" || links[2].DisplayText != "alias" {
		t.Fatalf("unexpected wikilink: %#v", links[2])
	}
	if links[3].AnchorType != "block" || links[3].AnchorValue != "block" {
		t.Fatalf("unexpected block anchor: %#v", links[3])
	}
}
