go get github.com/gin-gonic/gin


go get -u
go mod tidy

sudo lsof -i :5000
ps -Ao user,pid,command | grep -v grep | grep <PID>