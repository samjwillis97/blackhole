# TODO

Attempting to order these in what actually needs to be done

- [ ] Create events from files already in directory when starting monitors
    - Need to re-work how these actually work
    - [ ] Event based
        - Sonarr should redrive based off of files in both watch path and processing path
    - [ ] Poll based
- [ ] Check usage of `go` in the event watch handler
- [ ] Create a central HTTP client with:
    - [ ] retries
    - [ ] logging
- [ ] Confirm refresh of *arr after debrid mount symlinking
    - Unsure how to handle this
- [ ] Better logging
- [ ] Finish handling torrent files
- [ ] Write better comments in tests
- [ ] Fixup the Event based handlers `event.Name` it might be different on Darwin and Linux
- [ ] Notify *arr when an error occurs
- [ ] Check original file name for debrid mount handler, like the other scripts
- [ ] Use `cobra` to make command line entry point
- [ ] Think about how to use state from `GetInfo` to drive some things - would make it more reliable

- [X] Check if a file is still there before processing for Sonarr monitor due to debounce
- [X] When adding file to Debrid Monitor, check if the file already exists
- [X] Check if a file is still in processing before actually processing for Debrid mount monitor
- [X] Secret validation
- [X] Fixup all the error handled (search for `panic(err)`)
