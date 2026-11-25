package jade

import (
    "context"
    "crypto/hmac"
    "crypto/rand"
    "crypto/sha256"
    "embed"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "html/template"
    "io"
    "net"
    "net/http"
    "net/url"
    "os"
    "regexp"
    "strconv"
    "strings"
    "sync"
    "time"
)

//go:embed web/*
var web embed.FS
var pageTemplate=template.Must(template.New("post.html").Funcs(template.FuncMap{"slideNumber":func(i int)int{return i+1}}).ParseFS(web,"web/post.html"))

type Config struct { PublicURL string; Secret string; MediaMode string; InstagramDocID string }
func ConfigFromEnv()(Config,error){
    cfg:=Config{PublicURL:os.Getenv("JADE_PUBLIC_URL"),Secret:os.Getenv("JADE_SIGNING_SECRET"),MediaMode:os.Getenv("JADE_MEDIA_MODE"),InstagramDocID:os.Getenv("JADE_INSTAGRAM_DOC_ID")}
    if cfg.PublicURL=="" {if host:=os.Getenv("VERCEL_PROJECT_PRODUCTION_URL");host!=""{cfg.PublicURL="https://"+host}else if host=os.Getenv("VERCEL_URL");host!=""{cfg.PublicURL="https://"+host}else{cfg.PublicURL="http://localhost:8080"}}
    u,err:=url.Parse(cfg.PublicURL)
    if err!=nil||u.Host==""||u.User!=nil||u.RawQuery!=""||u.Fragment!=""||(u.Path!=""&&u.Path!="/"){return cfg,fmt.Errorf("JADE_PUBLIC_URL must be an HTTP(S) origin")}
    local:=u.Hostname()=="localhost"||u.Hostname()=="127.0.0.1"||u.Hostname()=="::1"
    if u.Scheme!="https"&&!(local&&u.Scheme=="http"){return cfg,fmt.Errorf("public deployments require HTTPS")}
    if len(cfg.Secret)<32 {if !local{return cfg,fmt.Errorf("set JADE_SIGNING_SECRET to at least 32 random characters")};key:=make([]byte,32);if _,err=rand.Read(key);err!=nil{return cfg,err};cfg.Secret=base64.RawURLEncoding.EncodeToString(key)}
    if cfg.MediaMode==""{cfg.MediaMode="proxy";if os.Getenv("VERCEL")=="1"{cfg.MediaMode="redirect"}}
    if cfg.MediaMode!="redirect"&&cfg.MediaMode!="proxy"{return cfg,fmt.Errorf("JADE_MEDIA_MODE must be redirect or proxy")}
    if cfg.InstagramDocID==""{cfg.InstagramDocID="27130156389949648"}
    cfg.PublicURL=strings.TrimSuffix(cfg.PublicURL,"/");return cfg,nil
}

type cacheEntry struct { post Post; err error; expires time.Time }
type flight struct { done chan struct{}; post Post; err error }
type App struct { cfg Config; resolver Resolver; client *http.Client; mu sync.Mutex; cache map[string]cacheEntry; pending map[string]*flight; limits map[string]requestCount }
type requestCount struct{ count int; reset time.Time }
func New(cfg Config,resolver Resolver)*App {
    client:=safeClient();if resolver==nil{resolver=&Providers{Client:client,InstagramDocID:cfg.InstagramDocID}}
    return &App{cfg:cfg,resolver:resolver,client:client,cache:map[string]cacheEntry{},pending:map[string]*flight{},limits:map[string]requestCount{}}
}
func (a *App) resolve(ctx context.Context,s Source)(Post,error){
    a.mu.Lock()
    if entry,ok:=a.cache[s.URL];ok&&time.Now().Before(entry.expires){a.mu.Unlock();return entry.post,entry.err}
    if current:=a.pending[s.URL];current!=nil {a.mu.Unlock();select{case <-current.done:return current.post,current.err;case <-ctx.Done():return Post{},ctx.Err()}}
    if len(a.pending)>=8{a.mu.Unlock();return Post{},fmt.Errorf("the service is busy; retry shortly")}
    current:=&flight{done:make(chan struct{})};a.pending[s.URL]=current;a.mu.Unlock()
    deadline,cancel:=context.WithTimeout(ctx,7*time.Second);defer cancel()
    post,err:=a.resolver.Resolve(deadline,s)
    if err==nil{post,err=validatePost(post)}
    a.mu.Lock();defer a.mu.Unlock()
    if len(a.cache)>=500 {for key:=range a.cache {delete(a.cache,key);break}}
    ttl:=5*time.Minute;if err!=nil{ttl=10*time.Second}
    a.cache[s.URL]=cacheEntry{post:post,err:err,expires:time.Now().Add(ttl)}
    current.post=post;current.err=err;delete(a.pending,s.URL);close(current.done)
    return post,err
}

func (a *App) allow(r *http.Request)bool{
    key:=r.RemoteAddr
    // Vercel overwrites this header. Do not trust forwarded headers elsewhere.
    if os.Getenv("VERCEL")=="1"{if ip:=r.Header.Get("X-Vercel-Forwarded-For");ip!=""{key=ip}}
    if host,_,err:=net.SplitHostPort(key);err==nil{key=host}
    a.mu.Lock();defer a.mu.Unlock();now:=time.Now();entry:=a.limits[key]
    if now.After(entry.reset){entry=requestCount{reset:now.Add(time.Minute)}};entry.count++;a.limits[key]=entry
    if len(a.limits)>10000{for old:=range a.limits{if old!=key{delete(a.limits,old);break}}}
    return entry.count<=300
}

func (a *App) ServeHTTP(w http.ResponseWriter,r *http.Request){
    w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("Referrer-Policy","no-referrer");w.Header().Set("Cache-Control","no-store")
    w.Header().Set("Content-Security-Policy","default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' https:; media-src 'self' https:; connect-src 'self'; base-uri 'none'; form-action 'self'; object-src 'none'")
    if r.Method!="GET"&&r.Method!="HEAD"{w.Header().Set("Allow","GET, HEAD");http.Error(w,"Use GET or HEAD",405);return}
    if len(r.URL.RequestURI())>4096{http.Error(w,"URL too long",414);return}
    path:=r.URL.Path
    if path=="/healthz" {writeJSON(w,r,200,map[string]any{"ok":true,"service":"jade"});return}
    if !a.allow(r){w.Header().Set("Retry-After","60");http.Error(w,"Too many requests",429);return}
    if path=="/"||path=="/style.css"||path=="/app.js"{
        name:=strings.TrimPrefix(path,"/");if name==""{name="index.html"};b,err:=web.ReadFile("web/"+name);if err!=nil{http.NotFound(w,r);return}
        w.Header().Set("Content-Type",map[string]string{"index.html":"text/html; charset=utf-8","style.css":"text/css; charset=utf-8","app.js":"text/javascript; charset=utf-8"}[name]);if r.Method!="HEAD"{_,_=w.Write(b)};return
    }
    if path=="/favicon.ico"{w.WriteHeader(204);return}
    if strings.HasPrefix(path,"/a/"){a.asset(w,r);return}
    var source Source;var err error
    api:=path=="/api/link"||path=="/api/resolve"
    if api{source,err=ParseSource(r.URL.Query().Get("url"))}else{source,err=routeSource(path)}
    if err!=nil{writeJSON(w,r,400,map[string]any{"error":"Use a public Instagram post, TikTok photo/video, or X post URL."});return}
    share:=a.cfg.PublicURL+source.SharePath()
    if path=="/api/link"{writeJSON(w,r,200,map[string]any{"share_url":share,"platform":source.Platform});return}
    post,err:=a.resolve(r.Context(),source)
    if err!=nil{writeJSON(w,r,502,map[string]any{"error":"Could not retrieve the post's complete media. It may be private, unavailable, rate-limited, or blocked by the platform."});return}
    links:=a.links(post)
    page:=1;if raw:=r.URL.Query().Get("page");raw!=""{page,err=strconv.Atoi(raw);if err!=nil||page<1{writeJSON(w,r,400,map[string]any{"error":"Invalid gallery page"});return}}
    gallery,err:=a.gallery(post,links,share,page)
    if err!=nil{writeJSON(w,r,400,map[string]any{"error":err.Error()});return}
    if path=="/api/resolve"{writeJSON(w,r,200,map[string]any{"share_url":share,"author":post.Author,"caption":post.Caption,"platform":source.Platform,"media":links,"gallery":gallery});return}
    encoded,_:=json.Marshal(gallery.Payload)
    view:=struct{Post Post;Links []Media;Share string;Gallery Gallery;Component template.JS;Fallback Media}{post,links,share,gallery,template.JS(encoded),links[gallery.Start-1]}
    w.Header().Set("Content-Type","text/html; charset=utf-8");w.Header().Set("Cache-Control","public, max-age=120");w.Header().Set("X-Robots-Tag","noindex")
    if r.Method!="HEAD"{_ = pageTemplate.Execute(w,view)}
}
func writeJSON(w http.ResponseWriter,r *http.Request,status int,data any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(status);if r.Method!="HEAD"{_=json.NewEncoder(w).Encode(data)}}

func (a *App) token(s Source)string{body:=base64.RawURLEncoding.EncodeToString([]byte(s.URL));mac:=hmac.New(sha256.New,[]byte(a.cfg.Secret));_,_=mac.Write([]byte(body));return body+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])}
func (a *App) verify(token string)(Source,error){
    parts:=strings.Split(token,".");if len(parts)!=2||len(token)>1800{return Source{},fmt.Errorf("invalid asset token")}
    sig,err:=base64.RawURLEncoding.DecodeString(parts[1]);if err!=nil{return Source{},err};mac:=hmac.New(sha256.New,[]byte(a.cfg.Secret));_,_=mac.Write([]byte(parts[0]));if !hmac.Equal(sig,mac.Sum(nil)[:16]){return Source{},fmt.Errorf("invalid asset token")}
    raw,err:=base64.RawURLEncoding.DecodeString(parts[0]);if err!=nil{return Source{},err};return ParseSource(string(raw))
}
func (a *App) links(post Post)[]Media{
    links:=make([]Media,len(post.Media));base:=a.cfg.PublicURL+"/a/"+a.token(post.Source)+"/"
    for i,m:=range post.Media{links[i]=m;ext:="jpg";if m.Kind=="video"{ext="mp4"};links[i].URL=base+strconv.Itoa(i)+"."+ext;if m.Poster!=""{links[i].Poster=base+strconv.Itoa(i)+".poster.jpg"}}
    return links
}

var assetPath=regexp.MustCompile(`^/a/([A-Za-z0-9_.-]+)/([0-9]{1,2})\.(mp4|jpg|poster\.jpg)$`)
var byteRange=regexp.MustCompile(`^bytes=(?:[0-9]+-[0-9]*|-[0-9]+)$`)
func (a *App) asset(w http.ResponseWriter,r *http.Request){
    match:=assetPath.FindStringSubmatch(r.URL.Path);if match==nil{http.NotFound(w,r);return};source,err:=a.verify(match[1]);if err!=nil{http.Error(w,"Invalid asset link",403);return}
    post,err:=a.resolve(r.Context(),source);if err!=nil{http.Error(w,"Media unavailable",502);return};index,_:=strconv.Atoi(match[2]);if index>=len(post.Media){http.NotFound(w,r);return}
    media:=post.Media[index];raw:=media.URL
    if match[3]=="poster.jpg"{raw=media.Poster}else if (match[3]=="mp4")!=(media.Kind=="video"){http.NotFound(w,r);return}
    if !mediaAllowed(raw,source.Platform){http.NotFound(w,r);return}
    if a.cfg.MediaMode=="redirect"{w.Header().Set("Cache-Control","no-store");http.Redirect(w,r,raw,http.StatusTemporaryRedirect);return}
    client:=*a.client;client.CheckRedirect=func(req *http.Request,via []*http.Request)error{if len(via)>3||!mediaAllowed(req.URL.String(),source.Platform){return fmt.Errorf("blocked media redirect")};return nil}
    req,err:=http.NewRequestWithContext(r.Context(),r.Method,raw,nil);if err!=nil{http.Error(w,"Media unavailable",502);return}
    req.Header.Set("User-Agent","Mozilla/5.0");req.Header.Set("Accept-Encoding","identity")
    if value:=r.Header.Get("Range");value!=""{if !byteRange.MatchString(value)||len(value)>80{http.Error(w,"Invalid range",416);return};req.Header.Set("Range",value)}
    response,err:=client.Do(req);if err!=nil{http.Error(w,"Media host unavailable",502);return};defer response.Body.Close()
    if response.StatusCode!=200&&response.StatusCode!=206{http.Error(w,"Media host refused this request",502);return}
    contentType:=strings.Split(response.Header.Get("Content-Type"),";")[0]
    allowed:=contentType=="image/jpeg"||contentType=="image/png"||contentType=="image/webp"||contentType=="image/avif"||contentType=="image/gif"
    if match[3]=="mp4"{allowed=contentType=="video/mp4"||contentType=="application/octet-stream";contentType="video/mp4"}
    limit:=int64(8<<20);if match[3]=="mp4"{limit=50<<20}
    if !allowed||response.ContentLength>limit||response.Header.Get("Content-Encoding")!=""{http.Error(w,"Unsupported or oversized media",502);return}
    w.Header().Set("Content-Type",contentType);w.Header().Set("Cache-Control","public, max-age=60");w.Header().Set("Access-Control-Allow-Origin","*")
    for _,name:=range []string{"Content-Length","Content-Range","Accept-Ranges"}{if value:=response.Header.Get(name);value!=""{w.Header().Set(name,value)}}
    w.WriteHeader(response.StatusCode);if r.Method!="HEAD"{_,_=io.Copy(w,io.LimitReader(response.Body,limit))}
}
