package handler

import (
    "net/http"
    "net/url"
    "sync"

    "github.com/AryaanSheth/jade/internal/jade"
)

var once sync.Once
var service *jade.App
var setupError error

// Handler is the standalone Vercel entry point. No bot session is started.
func Handler(w http.ResponseWriter,r *http.Request){
    once.Do(func(){var cfg jade.Config;cfg,setupError=jade.ConfigFromEnv();if setupError==nil{service=jade.New(cfg,nil)}})
    if setupError!=nil{http.Error(w,"Jade is not configured. Set JADE_PUBLIC_URL and JADE_SIGNING_SECRET.",503);return}
    clone:=r.Clone(r.Context());copyURL:=*r.URL;clone.URL=&copyURL
    query:=clone.URL.Query()
    if path:=query.Get("jade_path");path!=""{clone.URL.Path="/"+path;clone.URL.RawPath="";query.Del("jade_path");clone.URL.RawQuery=query.Encode()}
    // Keep the adapter independent of query ordering performed by Vercel.
    if clone.URL.Path=="/api/index.go"||clone.URL.Path=="/api/index"{clone.URL.Path="/"}
    if _,err:=url.ParseRequestURI(clone.URL.RequestURI());err!=nil{http.Error(w,"Invalid path",400);return}
    service.ServeHTTP(w,clone)
}
