package sources

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestFindJobPosting(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string // title of the found posting, "" when none
	}{
		{
			name: "direct object",
			body: `<html><head><script type="application/ld+json">
			  {"@context":"https://schema.org","@type":"JobPosting","title":"Direct Role"}
			</script></head></html>`,
			want: "Direct Role",
		},
		{
			name: "array of nodes",
			body: `<html><head><script type="application/ld+json">
			  [{"@type":"Organization","name":"Acme"},{"@type":"JobPosting","title":"Array Role"}]
			</script></head></html>`,
			want: "Array Role",
		},
		{
			name: "graph wrapper",
			body: `<html><head><script type="application/ld+json">
			  {"@context":"https://schema.org","@graph":[
			    {"@type":"WebSite","name":"Site"},
			    {"@type":["JobPosting"],"title":"Graph Role"}
			  ]}
			</script></head></html>`,
			want: "Graph Role",
		},
		{
			name: "no job posting",
			body: `<html><head><script type="application/ld+json">
			  {"@type":"Organization","name":"Acme"}
			</script></head></html>`,
			want: "",
		},
		{
			name: "malformed json is skipped",
			body: `<html><head><script type="application/ld+json">{not json}</script></head></html>`,
			want: "",
		},
		{
			name: "non json-ld script is ignored",
			body: `<html><head><script type="text/javascript">
			  {"@type":"JobPosting","title":"Ignored"}
			</script></head></html>`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			posting := FindJobPosting(doc)
			got := ""
			if posting != nil {
				got, _ = posting["title"].(string)
			}
			if got != tt.want {
				t.Fatalf("FindJobPosting title = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindJobPostingNilDoc(t *testing.T) {
	if posting := FindJobPosting(nil); posting != nil {
		t.Fatalf("FindJobPosting(nil) = %v, want nil", posting)
	}
}
