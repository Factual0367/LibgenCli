package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/table"
)

func TestCleanFileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test File.txt", "Test_File.txt"},
		{"Test/File.txt", "Test_File.txt"},
		{"Test File@123.txt", "Test_File123.txt"},
		{"Test#File.txt", "TestFile.txt"},
	}

	for _, tt := range tests {
		result := CleanFileName(tt.input)
		if result != tt.expected {
			t.Errorf("CleanFileName(%s) = %s; expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestDownloadFileSuccess(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("File content"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	title := "Test_File"
	filetype := "txt"
	link := server.URL

	err := DownloadFile(title, filetype, link)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	filename := CleanFileName(title + "." + filetype)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Errorf("Expected file %s to exist, but it does not", filename)
	}

	os.Remove(filename)
}

func TestDownloadFileFail(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	title := "Test_File"
	filetype := "txt"
	link := server.URL

	err := DownloadFile(title, filetype, link)
	if err == nil {
		t.Fatalf("Expected error, got none")
	}
}

func TestUpdateRowStatus(t *testing.T) {
	rows := []table.Row{
		{"Author1", "Title1", "pdf", "link1", ""},
		{"Author2", "Title2", "epub", "link2", ""},
	}

	updatedRows := updateRowStatus(rows, 1, "Downloading...")
	expectedStatus := "Downloading..."

	if updatedRows[1][4] != expectedStatus {
		t.Errorf("Expected status %s, but got %s", expectedStatus, updatedRows[1][4])
	}
}
