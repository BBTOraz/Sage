package chunking

import (
	"strings"
	"testing"
)

func TestBuildPassportExtractsLanguageTermsAndAcronyms(t *testing.T) {
	content := strings.Join([]string{
		"Spring Security Web Security Features and Protections",
		"Cross-Site Request Forgery (CSRF) is a web attack mitigated by Spring Security.",
		"CSRF protection relies on synchronizer tokens and server-side validation.",
		"Content Security Policy (CSP) helps reduce cross-site scripting risk.",
		"Spring Security filter chain protects the application request lifecycle.",
	}, "\n\n")

	passport := buildPassport(content, []textBlock{
		{Text: "Spring Security Web Security Features and Protections", Kind: blockHeading},
		{Text: "Cross-Site Request Forgery (CSRF)", Kind: blockHeading},
		{Text: "Content Security Policy (CSP)", Kind: blockHeading},
	})

	if passport.Language != "en" {
		t.Fatalf("expected english passport, got %q", passport.Language)
	}
	if passport.DocumentType != "guide" {
		t.Fatalf("expected guide document type, got %q", passport.DocumentType)
	}
	if !containsString(passport.Acronyms, "CSRF") || !containsString(passport.Acronyms, "CSP") {
		t.Fatalf("expected acronyms to be extracted, got %+v", passport.Acronyms)
	}
	if !containsString(passport.Aliases, "Cross-Site Request Forgery") {
		t.Fatalf("expected alias to include long form, got %+v", passport.Aliases)
	}
	if !containsString(passport.KeyPhrases, "Cross-Site Request Forgery") {
		t.Fatalf("expected key phrase from heading, got %+v", passport.KeyPhrases)
	}
	if !containsString(passport.TopTerms, "spring") {
		t.Fatalf("expected top terms to include spring, got %+v", passport.TopTerms)
	}
}

func TestBuildPassportClassifiesReferenceDocuments(t *testing.T) {
	content := strings.Join([]string{
		"Android Developer Documentation: Jetpack Compose Overview. URL: https://developer.android.com",
		"Firebase Documentation: Cloud Firestore. URL: https://firebase.google.com/docs/firestore",
		"Kotlin in Action. Manning Publications.",
		"Android Application Security Essentials. Pragmatic Bookshelf.",
	}, "\n")

	passport := buildPassport(content, nil)
	if passport.DocumentType != "reference" {
		t.Fatalf("expected reference document type, got %q", passport.DocumentType)
	}
	if passport.Language != "en" {
		t.Fatalf("expected english passport, got %q", passport.Language)
	}
}

func TestBuildPassportDetectsRussianText(t *testing.T) {
	content := "Защита баз данных и управление доступом в распределенных системах."
	passport := buildPassport(content, nil)
	if passport.Language != "ru" {
		t.Fatalf("expected russian passport, got %q", passport.Language)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
