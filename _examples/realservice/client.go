package main

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/mgo.v2/bson"

	"github.com/smallnest/rpcx"
)

type Args struct {
	PostType string `msg:"posttype"`
}

type Reply struct {
	Posts []Post `msg:"posts"`
}

type Post struct {
	PostID      bson.ObjectId `json:"id" xml:"id" bson:"_id,omitempty"`
	PostType    string        `json:"ptype" xml:"ptype" bson:"ptype,omitempty"`
	Title       string        `json:"title" xml:"title" bson:"title"`
	URL         string        `json:"url" xml:"url" bson:"url"`
	Domain      string        `json:"domain" xml:"domain" bson:"domain"`
	ShortURL    string        `json:"surl" xml:"surl" bson:"surl"`
	Description string        `json:"desc" xml:"desc" bson:"desc"`
	LikeCount   int           `json:"like" xml:"like" bson:"like"`
	ImageURL    string        `json:"imgurl" xml:"imgurl" bson:"imgurl"`
	RecommendBy string        `json:"-" xml:"-" bson:"-"`
	Tags        string        `json:"tags" xml:"tags" bson:"tags"`
	State       int           `json:"-" xml:"-" bson:"-"`
	Timestamp   time.Time     `json:"ts" xml:"timestamp" bson:"ts"`
}

func main() {
	s := &rpcx.DirectClientSelector{Network: "tcp", Address: "tr.colobu.com:8972", DialTimeout: 10 * time.Second}
	client := rpcx.NewClient(s)
	defer client.Close()

	args := &Args{"golang"}
	var reply Reply
	err := client.Call("Posts.Query", args, &reply)
	if err != nil {
		fmt.Printf("error for Posts: %s, %v \n", args.PostType, err)
		return
	}

	posts := reply.Posts
	data, _ := json.MarshalIndent(&posts, "", "\t")

	fmt.Printf("Posts: %s \n", string(data))
}
