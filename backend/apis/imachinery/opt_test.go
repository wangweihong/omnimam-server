package imachinery

import (
	"math"
	"reflect"
	"testing"
)

func TestPagingParamsNormalize(t *testing.T) {
	tests := []struct {
		name    string
		params  PagingParams
		want    PageWindow
		wantErr bool
	}{
		{name: "omitted values use default", params: PagingParams{}, want: PageWindow{Offset: 0, Limit: 500}},
		{name: "first explicit page", params: PagingParams{PageNum: 0, PageSize: 20}, want: PageWindow{Offset: 0, Limit: 20}},
		{name: "second explicit page", params: PagingParams{PageNum: 1, PageSize: 20}, want: PageWindow{Offset: 20, Limit: 20}},
		{name: "maximum page size", params: PagingParams{PageNum: 2, PageSize: 500}, want: PageWindow{Offset: 1000, Limit: 500}},
		{name: "oversized page is capped", params: PagingParams{PageNum: 2, PageSize: 900}, want: PageWindow{Offset: 1000, Limit: 500}},
		{name: "negative page number", params: PagingParams{PageNum: -1}, wantErr: true},
		{name: "negative page size", params: PagingParams{PageSize: -1}, wantErr: true},
		{name: "offset overflow", params: PagingParams{PageNum: math.MaxInt, PageSize: 2}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.params.Normalize()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Normalize() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPaginateSlice(t *testing.T) {
	items := []int{0, 1, 2, 3, 4}
	tests := []struct {
		name   string
		window PageWindow
		want   []int
	}{
		{name: "first page", window: PageWindow{Offset: 0, Limit: 2}, want: []int{0, 1}},
		{name: "partial final page", window: PageWindow{Offset: 4, Limit: 2}, want: []int{4}},
		{name: "page out of range", window: PageWindow{Offset: 5, Limit: 2}, want: []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PaginateSlice(items, tt.window); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("PaginateSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}
