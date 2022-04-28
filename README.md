go get github.com/gin-gonic/gin
go get go.mongodb.org/mongo-driver/mongo
go get github.com/streadway/amqp

go get -u
go mod tidy

sudo lsof -i :5000
ps -Ao user,pid,command | grep -v grep | grep <PID>

https://www.reddit.com/r/redditdev/comments/t8e8hc/getting_nothing_but_429_responses_when_using_go/

export MONGO_DATABASE=reddit
export MONGO_URI=mongodb://localhost:27017/test
export RABBITMQ_URI="amqp://user:password@localhost:5672/" 
export RABBITMQ_QUEUE=rss_urls

docker run -d --name rabbitmq -e RABBITMQ_DEFAULT_USER=user -e RABBITMQ_DEFAULT_PASS=password -p 8080:15672 -p 5672:5672 rabbitmq:3-management