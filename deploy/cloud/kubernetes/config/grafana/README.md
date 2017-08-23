
The service dashboard config file is templated to facilitate easy changes. See 
`dashboard.service.json.template`. Regenerate `dashboard.service.json` via

    go run gen.go
    
Since we don't expect `dashboard.service.json` to change too much, it's committed to the repo.