package engine

import (
	"reflect"
	"testing"
)

func TestMergeSymbols(t *testing.T) {
	tests := []struct {
		name    string
		primary []string
		extra   []string
		want    []string
	}{
		{"both empty", nil, nil, []string{}},
		{"primary only", []string{"XRP_JPY", "XLM_JPY"}, nil, []string{"XRP_JPY", "XLM_JPY"}},
		{"extra only", nil, []string{"MONA_JPY"}, []string{"MONA_JPY"}},
		{
			"union dedupes overlap",
			[]string{"XRP_JPY", "XLM_JPY"},
			[]string{"XLM_JPY", "MONA_JPY"},
			[]string{"XRP_JPY", "XLM_JPY", "MONA_JPY"},
		},
		{
			"skip empty entries",
			[]string{"", "XRP_JPY"},
			[]string{"", "MONA_JPY", ""},
			[]string{"XRP_JPY", "MONA_JPY"},
		},
		{
			"primary order preserved",
			[]string{"B", "A"},
			[]string{"C", "A"},
			[]string{"B", "A", "C"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeSymbols(tt.primary, tt.extra)
			if len(tt.want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeSymbols(%v, %v) = %v, want %v", tt.primary, tt.extra, got, tt.want)
			}
		})
	}
}
