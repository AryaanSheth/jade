package jade

import (
    "fmt"
    "net/url"
    "regexp"
    "strings"
)

type Source struct { Platform string `json:"platform"`; URL string `json:"url"`; Path string `json:"-"`; ID string `json:"-"`; Short bool `json:"-"` }
var instagramPath = regexp.MustCompile(`^/(p|reel|reels|tv)/([A-Za-z0-9_-]{3,80})/?$`)
var tiktokPath = regexp.MustCompile(`^/@([A-Za-z0-9_.]{1,30})/(video|photo)/([0-9]{5,30})/?$`)
var shortPath = regexp.MustCompile(`^/(?:t/)?[A-Za-z0-9_-]{3,100}/?$`)
var tweetPath = regexp.MustCompile(`^/(?:[A-Za-z0-9_]{1,15}/status|i/web/status|i/status|statuses)/([0-9]{5,30})(?:/(?:photo|video)/[0-9]{1,2})?/?$`)

func ParseSource(raw string) (Source, error) {
    bad := fmt.Errorf("use a public Instagram post, TikTok post, or X/Twitter status link")
    if len(raw) > 1800 || strings.ContainsAny(raw, "\\ \r\n\t") { return Source{}, bad }
    if !strings.Contains(raw, "://") { raw = "https://"+raw }
    u, err := url.Parse(raw)
    if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "https" && u.Scheme != "http") { return Source{}, bad }
    host := strings.ToLower(u.Hostname())
    path := u.EscapedPath()
    var s Source
    switch host {
    case "instagram.com", "www.instagram.com":
        m := instagramPath.FindStringSubmatch(path); if m == nil { return s, bad }
        kind := m[1]; if kind == "reels" { kind = "reel" }
        s = Source{Platform:"instagram", Path:"/"+kind+"/"+m[2]+"/", ID:m[2]}
        s.URL = "https://www.instagram.com"+s.Path
    case "tiktok.com", "www.tiktok.com", "m.tiktok.com", "vm.tiktok.com", "vt.tiktok.com":
        m := tiktokPath.FindStringSubmatch(path)
        if m != nil { s = Source{Platform:"tiktok", Path:"/@"+m[1]+"/"+m[2]+"/"+m[3], ID:m[3]}; s.URL="https://www.tiktok.com"+s.Path
        } else if shortPath.MatchString(path) && (host=="vm.tiktok.com" || host=="vt.tiktok.com" || strings.HasPrefix(path,"/t/")) {
            s=Source{Platform:"tiktok",URL:"https://"+host+path,Path:"/"+host+path,Short:true}
        } else { return s,bad }
    case "x.com", "www.x.com", "mobile.x.com", "twitter.com", "www.twitter.com", "mobile.twitter.com":
        m:=tweetPath.FindStringSubmatch(path); if m==nil { return s,bad }
        // A media selector still shares the complete post, including every slide.
        s=Source{Platform:"x",Path:"/i/status/"+m[1],ID:m[1]}; s.URL="https://x.com"+s.Path
    default: return s,bad
    }
    return s,nil
}

func (s Source) SharePath() string { if s.Short { return s.Path }; return "/"+s.Platform+s.Path }

func routeSource(path string) (Source,error) {
    if strings.HasPrefix(path,"/https://") || strings.HasPrefix(path,"/http://") { return ParseSource(path[1:]) }
    for prefix, host := range map[string]string{"/instagram/":"www.instagram.com", "/tiktok/":"www.tiktok.com", "/x/":"x.com", "/twitter/":"x.com"} {
        if strings.HasPrefix(path,prefix) { return ParseSource("https://"+host+path[len(prefix)-1:]) }
    }
    return ParseSource(strings.TrimPrefix(path,"/"))
}
