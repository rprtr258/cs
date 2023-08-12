package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreParseQuery(t *testing.T) {
	for name, test := range map[string]struct {
		args         []string
		wantModified []string
		wantFuzzy    string
	}{
		"empty": {
			args:         []string{},
			wantModified: []string{},
			wantFuzzy:    "",
		},
		"no fuzzy": {
			args:         []string{"test"},
			wantModified: []string{"test"},
			wantFuzzy:    "",
		},
		"single fuzzy": {
			args:         []string{"file:test"},
			wantModified: []string{},
			wantFuzzy:    "test",
		},
		"single fuzzy alternate": {
			args:         []string{"filename:test"},
			wantModified: []string{},
			wantFuzzy:    "test",
		},
		"multi fuzzy last wins": {
			args:         []string{"file:test", "file:other"},
			wantModified: []string{},
			wantFuzzy:    "other",
		},
		"single fuzzy single term": {
			args:         []string{"stuff", "file:test"},
			wantModified: []string{"stuff"},
			wantFuzzy:    "test",
		},
		"single fuzzy uppercase": {
			args:         []string{"FILE:test", "UPPER"},
			wantModified: []string{"UPPER"},
			wantFuzzy:    "test",
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			modified, fuzzy := PreParseQuery(test.args)
			assert.Equal(t, test.wantModified, modified)
			assert.Equal(t, test.wantFuzzy, fuzzy)
		})
	}
}
