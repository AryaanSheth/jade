package handler

import (
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"
)

func TestRewriteRoutesToStandaloneService(t *testing.T){
    t.Setenv("JADE_PUBLIC_URL","https://jade.example.com")
    t.Setenv("JADE_SIGNING_SECRET",strings.Repeat("s",64))
    path:="/api/index.go?jade_path=api%2Flink&url="+url.QueryEscape("https://x.com/creator/status/1234567890123456789")
    w:=httptest.NewRecorder();Handler(w,httptest.NewRequest("GET",path,nil))
    if w.Code!=200||!strings.Contains(w.Body.String(),"https://jade.example.com/x/i/status/1234567890123456789"){t.Fatal(w.Code,w.Body.String())}
    w=httptest.NewRecorder();Handler(w,httptest.NewRequest("GET","/api/index.go?jade_path=",nil))
    if w.Code!=200||!strings.Contains(w.Body.String(),"Just A Discord Embedder"){t.Fatal(w.Code,w.Body.String())}
}
