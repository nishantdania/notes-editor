package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	// Default values
	defaultDir, _ := os.UserHomeDir()
	defaultPort := 8013
	
	// Get directory from environment variable, use default if not set
	notesDir := os.Getenv("NOTES_DIR")
	if notesDir == "" {
		notesDir = defaultDir
	}
	
	// Get port from environment variable, use default if not set or invalid
	portStr := os.Getenv("NOTES_PORT")
	port := defaultPort
	if portStr != "" {
		if parsedPort, err := strconv.Atoi(portStr); err == nil {
			port = parsedPort
		} else {
			fmt.Printf("Warning: Invalid port number '%s', using default port %d\n", portStr, defaultPort)
		}
	}
	
	// Create path to notes.txt
	notesFile := filepath.Join(notesDir, "notes.txt")
	backupDir := filepath.Join(notesDir, "notes_backups")
	fmt.Printf("Using notes file: %s\n", notesFile)
	fmt.Printf("Using backup directory: %s\n", backupDir)
	fmt.Printf("Starting server on port: %d\n", port)
	
	// Main handler for the notes editor
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			// Create directory if it doesn't exist
			os.MkdirAll(filepath.Dir(notesFile), 0755)
			
			// If the notes file exists, create a timestamped backup before saving
			if _, err := os.Stat(notesFile); !os.IsNotExist(err) {
				content, err := ioutil.ReadFile(notesFile)
				if err == nil {
					// Create backup directory if it doesn't exist
					os.MkdirAll(backupDir, 0755)
					
					// Create timestamped backup filename
					timestamp := time.Now().Format("20060102_150405")
					backupFile := filepath.Join(backupDir, fmt.Sprintf("notes_%s.bak", timestamp))
					
					// Save backup
					ioutil.WriteFile(backupFile, content, 0644)
					
					// Clean up old backups (older than 1 hour)
					cleanupOldBackups(backupDir, 1*time.Hour)
				}
			}
			
			// Save the content to notes.txt
			ioutil.WriteFile(notesFile, []byte(r.FormValue("content")), 0644)
			
			// For AJAX requests, return the last modification time
			if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
				fileInfo, err := os.Stat(notesFile)
				lastModTime := ""
				if err == nil {
					lastModTime = fileInfo.ModTime().Format("2006-01-02 15:04:05")
				}
				w.Write([]byte(lastModTime))
				return
			}
			
			http.Redirect(w, r, "/", 302)
			return
		}
		
		// Create file if it doesn't exist
		if _, err := os.Stat(notesFile); os.IsNotExist(err) {
			// Make sure the directory exists
			os.MkdirAll(filepath.Dir(notesFile), 0755)
			ioutil.WriteFile(notesFile, []byte(""), 0644)
		}
		
		// Read file content
		content, err := ioutil.ReadFile(notesFile)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Error reading notes file: %v", err)))
			return
		}
		
		// Get the file's last modification time
		fileInfo, err := os.Stat(notesFile)
		lastModTime := ""
		if err == nil {
			lastModTime = fileInfo.ModTime().Format("2006-01-02 15:04:05")
		}
		
		// Get list of backup files for dropdown
		backupFiles := []string{}
		backupCount := 0
		if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
			files, err := ioutil.ReadDir(backupDir)
			if err == nil {
				backupCount = len(files)
				// Reverse the files to show newest first
				for i := len(files) - 1; i >= 0; i-- {
					if !files[i].IsDir() && strings.HasSuffix(files[i].Name(), ".bak") {
						// Format the time from the filename for display
						name := files[i].Name()
						// Try to extract and format the date part if possible
						if len(name) > 11 { // "notes_" + 8 chars for date + "_" + 6 chars for time + ".bak"
							dateStr := name[6:14] // Extract "YYYYMMDD" part
							timeStr := name[15:21] // Extract "HHMMSS" part
							displayTime := ""
							
							t, err := time.Parse("20060102_150405", dateStr+"_"+timeStr)
							if err == nil {
								displayTime = t.Format("Jan 2, 2006 at 15:04:05")
							} else {
								displayTime = name
							}
							
							backupFiles = append(backupFiles, fmt.Sprintf(`<option value="%s">%s</option>`, files[i].Name(), displayTime))
						} else {
							backupFiles = append(backupFiles, fmt.Sprintf(`<option value="%s">%s</option>`, files[i].Name(), name))
						}
					}
				}
			}
		}
		
		// Create backup file dropdown HTML
		backupDropdown := ""
		if len(backupFiles) > 0 {
			backupDropdown = `
				<div style="margin-top: 10px;">
					<label for="backupSelect">View backup: </label>
					<select id="backupSelect">
						<option value="">Select a backup</option>
						` + strings.Join(backupFiles, "\n") + `
					</select>
					<button type="button" id="viewBackupBtn">View</button>
				</div>
			`
		}
		
		// Serve the editor with autosave functionality
		w.Write([]byte(`
			<html>
			<head>
				<title>Notes</title>
				<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
				<script>
					// Debounce function to limit how often autosave is triggered
					function debounce(func, wait) {
						let timeout;
						return function(...args) {
							const context = this;
							clearTimeout(timeout);
							timeout = setTimeout(() => func.apply(context, args), wait);
						};
					}
					
					// Setup when the document is loaded
					document.addEventListener('DOMContentLoaded', function() {
						const textarea = document.querySelector('textarea[name="content"]');
						const saveStatus = document.getElementById('saveStatus');
						
						// Set initial save status
						saveStatus.textContent = 'Last saved: ` + lastModTime + `';
						
						// Create debounced save function
						const saveNotes = debounce(function() {
							saveStatus.textContent = 'Saving...';
							
							// Create AJAX request to save content
							const xhr = new XMLHttpRequest();
							xhr.open('POST', '/', true);
							xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
							xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
							
							xhr.onreadystatechange = function() {
								if (xhr.readyState === 4) {
									if (xhr.status === 200) {
										saveStatus.textContent = 'Last saved: ' + xhr.responseText;
									} else {
										saveStatus.textContent = 'Error saving!';
									}
								}
							};
							
							xhr.send('content=' + encodeURIComponent(textarea.value));
						}, 250); // Save after 250 milisecond of edits
						
						// Add event listener for typing
						textarea.addEventListener('input', function() {
							saveStatus.textContent = 'Typing...';
							saveNotes();
						});
						
						// Still allow manual saving
						document.querySelector('form').addEventListener('submit', function(e) {
							saveStatus.textContent = 'Saving...';
						});
						
						// Set up backup viewer button
						const viewBackupBtn = document.getElementById('viewBackupBtn');
						if (viewBackupBtn) {
							viewBackupBtn.addEventListener('click', function() {
								const selectElem = document.getElementById('backupSelect');
								const selectedBackup = selectElem.value;
								if (selectedBackup) {
									window.open('/backup/' + encodeURIComponent(selectedBackup), '_blank');
								}
							});
						}
					});
				</script>
				<style>
					#saveStatus {
						color: #666;
						font-style: italic;
						margin-left: 10px;
					}
					textarea {
						width: 100%;
						height: 50vh;
						font-family: monospace;
						font-size: 16px;
					}
				</style>
			</head>
			<body>
				<form method="post">
					<textarea name="content">` + string(content) + `</textarea>
					<p>
						<button>Save</button>
						<small>(Autosaves after 250 seconds of edits)</small>
					</p>
					<p>Editing: ` + notesFile + ` <span id="saveStatus">Last saved: ` + lastModTime + `</span></p>
					<p><small>` + strconv.Itoa(backupCount) + ` backups available in ` + backupDir + ` (keeping the last hour of changes)</small></p>
				</form>
				` + backupDropdown + `
			</body>
			</html>
		`))
	})
	
	// Handler for viewing backup files
	http.HandleFunc("/backup/", func(w http.ResponseWriter, r *http.Request) {
		// Extract the backup filename from the URL
		backupFileName := filepath.Base(r.URL.Path)
		
		// Security check - make sure the backup filename doesn't contain path traversal
		if strings.Contains(backupFileName, "..") || strings.Contains(backupFileName, "/") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid backup filename"))
			return
		}
		
		// Ensure the file has .bak extension
		if !strings.HasSuffix(backupFileName, ".bak") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid backup file type"))
			return
		}
		
		// Create the full path to the backup file
		backupFilePath := filepath.Join(backupDir, backupFileName)
		
		// Check if the file exists
		if _, err := os.Stat(backupFilePath); os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Backup file not found"))
			return
		}
		
		// Read the backup file
		content, err := ioutil.ReadFile(backupFilePath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Error reading backup file: %v", err)))
			return
		}
		
		// Extract date from filename for display
		displayDate := backupFileName
		if len(backupFileName) > 11 { // "notes_" + 8 chars for date + "_" + 6 chars for time + ".bak"
			dateStr := backupFileName[6:14] // Extract "YYYYMMDD" part
			timeStr := backupFileName[15:21] // Extract "HHMMSS" part
			
			t, err := time.Parse("20060102_150405", dateStr+"_"+timeStr)
			if err == nil {
				displayDate = t.Format("January 2, 2006 at 15:04:05")
			}
		}
		
		// Serve the backup viewer
		w.Write([]byte(`
			<html>
			<head>
				<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
				<title>Backup: ` + displayDate + `</title>
				<style>
					textarea {
						width: 100%;
						height: 50vh;
						font-family: monospace;
					}
				</style>
			</head>
			<body>
				<h1>Backup from ` + displayDate + `</h1>
				<div class="buttons">
					<button onclick="window.close()">Close</button>
					<button onclick="window.location.href='/'">Back to Editor</button>
					<button id="restoreBtn">Restore This Version</button>
				</div>
				<textarea readonly>` + string(content) + `</textarea>
				
				<script>
					document.getElementById('restoreBtn').addEventListener('click', function() {
						if (confirm('Are you sure you want to restore this backup? Current content will be overwritten.')) {
							// Create form and submit POST to main page
							const form = document.createElement('form');
							form.method = 'POST';
							form.action = '/';
							
							const input = document.createElement('input');
							input.type = 'hidden';
							input.name = 'content';
							input.value = document.querySelector('textarea').value;
							
							form.appendChild(input);
							document.body.appendChild(form);
							form.submit();
						}
					});
				</script>
			</body>
			</html>
		`))
	})
	
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// cleanupOldBackups removes backup files older than the specified duration
func cleanupOldBackups(backupDir string, maxAge time.Duration) {
	files, err := ioutil.ReadDir(backupDir)
	if err != nil {
		return
	}
	
	cutoffTime := time.Now().Add(-maxAge)
	
	for _, file := range files {
		if !file.IsDir() && file.ModTime().Before(cutoffTime) {
			os.Remove(filepath.Join(backupDir, file.Name()))
		}
	}
}
