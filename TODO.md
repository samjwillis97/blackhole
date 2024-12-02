# TODO

Attempting to order these in what actually needs to be done

- [ ] Check if a file is still in processing before actually processing for Debrid mount monitor
- [ ] Check if a file is still there before processing for Sonarr monitor due to debounce
- [ ] When adding file to Debrid Monitor, check if the file already exists
- [ ] Check usage of `go` in the event watch handler
- [ ] Create events from files already in directory when starting monitors
    - [ ] Event based
    - [ ] Poll based
- [ ] Create a central HTTP client with:
    - [ ] retries
    - [ ] logging
- [ ] Secret validation
- [ ] Better logging
- [ ] Write better comments in tests
- [ ] Fixup the Event based handlers `event.Name` it might be different on Darwin and Linux
- [ ] Fixup all the error handled (search for `panic(err)`)
- [ ] Check original file name for debrid mount handler, like the other scripts
- [ ] Use `cobra` to make command line entry point
