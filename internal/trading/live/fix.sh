#!/bin/bash
sed -i '' 's/(s \*Service)/(t \*Trader)/g' internal/trading/live/service.go
sed -i '' 's/\*Service/\*Trader/g' internal/trading/live/service.go
sed -i '' 's/\bs\./t./g' internal/trading/live/service.go

