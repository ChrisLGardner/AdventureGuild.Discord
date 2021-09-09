package main

import (
	"reflect"
	"testing"
	"time"
)

func TestJob_parseString(t *testing.T) {
	expDate, _ := time.Parse("2006-01-02 15:04", "2006-01-02 17:00")

	type fields struct {
		Title       string
		Date        time.Time
		Description string
	}
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Valid job with empty first line",
			fields: fields{
				Title:       "Test",
				Date:        expDate,
				Description: "Test string\ngoes here",
			},
			args: args{
				s: "\nTest\n2006-01-02 17:00\nTest string\ngoes here",
			},
			wantErr: false,
		},
		{
			name: "Valid job with normal first line",
			fields: fields{
				Title:       "Test",
				Date:        expDate,
				Description: "Test string\ngoes here",
			},
			args: args{
				s: "Test\n2006-01-02 17:00\nTest string\ngoes here",
			},
			wantErr: false,
		},
		{
			name: "Invalid job with bad date",
			fields: fields{
				Title:       "Test",
				Date:        expDate,
				Description: "Test string\ngoes here",
			},
			args: args{
				s: "\nTest\nJan 1 2021 5PM\nTest string\ngoes here",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &Job{}
			if err := j.parseString(tt.args.s); (err != nil) != tt.wantErr {
				t.Fatalf("Job.parseString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if j.Title != tt.fields.Title {
					t.Errorf("Job.parseString() expected title = %v, got title = %v", tt.fields.Title, j.Title)
				}
				if j.Date != tt.fields.Date {
					t.Errorf("Job.parseString() expected Date = %v, got Date = %v", tt.fields.Date, j.Date)
				}
				if j.Description != tt.fields.Description {
					t.Errorf("Job.parseString() expected Description = %v, got Description = %v", tt.fields.Description, j.Description)
				}
			}
		})
	}
}

func TestJob_parseDate(t *testing.T) {
	expDate, _ := time.Parse("2006-01-02 15:04", "2021-01-01 17:00")
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    time.Time
		wantErr bool
	}{
		{
			name: "Year-Month-Day date with hyphens and 24 hour time",
			args: args{
				s: "2021-01-01 17:00",
			},
			want:    expDate,
			wantErr: false,
		},
		{
			name: "Year-Month-Day date with hyphens and 12 hour time",
			args: args{
				s: "2021-01-01 05:00PM",
			},
			want:    expDate,
			wantErr: false,
		},
		{
			name: "Year-Month-Day date with slashes and 24 hour time",
			args: args{
				s: "2021/01/01 17:00",
			},
			want:    expDate,
			wantErr: false,
		},
		{
			name: "Year-Month-Day  date with slashes and 12 hour time",
			args: args{
				s: "2021/01/01 05:00PM",
			},
			want:    expDate,
			wantErr: false,
		},
		{
			name: "Day-Month-Year date with hyphens and 24 hour time",
			args: args{
				s: "01-01-2021 17:00",
			},
			want:    expDate,
			wantErr: false,
		},
		{
			name: "Day-Month-Year date with hyphens and 12 hour time",
			args: args{
				s: "01-01-2021 05:00PM",
			},
			want:    expDate,
			wantErr: false,
		},
		{
			name: "Day-Month-Year date with slashes and 24 hour time",
			args: args{
				s: "01/01/2021 17:00",
			},
			want:    expDate,
			wantErr: false,
		},
		{
			name: "Day-Month-Year date with slashes and 12 hour time",
			args: args{
				s: "01/01/2021 05:00PM",
			},
			want:    expDate,
			wantErr: false,
		},
		{
			name: "Invalid date",
			args: args{
				s: "Jan 1 2021 5PM",
			},
			want:    time.Time{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDate(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("Job.parseDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Job.parseDate() = %v, want %v", got, tt.want)
			}
			if tt.wantErr && err.Error() != "invalid date format provided, please provide date in the format Year-Month-Day 24:00" {
				t.Errorf("Job.parseDate() error = %v, wantErr %v", err, "invalid date format provided, please provide date in the format Year-Month-Day 24:00")
			}
		})
	}
}
