# Adding a command
 - make a new subpackage for it
 - define `Register(...)`
 - include RegisterReqs and Managers to the BotCommandPackage if necessary and insert in main
 - import to `init.go` and call `Register(...)`