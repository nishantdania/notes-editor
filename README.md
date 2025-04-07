Notes Editor

A minimal web based notes editor for a single txt file based notes system.

---

Running as systemd service:

```bash
go build -o notes-editor main.go
```

```bash
sudo mv notes-editor /usr/local/bin/
sudo chmod +x /usr/local/bin/notes-editor
```

```bash
sudo nano /etc/systemd/system/notes-editor.service
```

```
[Unit]
Description=Web-based Notes Editor
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
Environment="NOTES_DIR=/home/YOUR_USERNAME"
Environment="NOTES_PORT=8080"
ExecStart=/usr/local/bin/notes-editor
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable notes-editor.service
sudo systemctl start notes-editor.service
```

```bash
sudo systemctl status notes-editor.service
```
