package main

import (
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
				s: "\nTest\n2006/01/02 17:00\nTest string\ngoes here",
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
