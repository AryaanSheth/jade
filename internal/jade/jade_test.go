package jade

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/http/httptest"
    "net/netip"
    "net/url"
    "regexp"
    "strings"
    "sync"
    "sync/atomic"
    "testing"
    "time"
)

var componentPattern=regexp.MustCompile(`(?s)<script id="discord:component-embed" type="application/json">(.*?)</script>`)
type resolverFunc func(context.Context,Source)(Post,error)
func(f resolverFunc)Resolve(ctx context.Context,s Source)(Post,error){return f(ctx,s)}
type transportFunc func(*http.Request)(*http.Response,error)
func(f transportFunc)RoundTrip(r *http.Request)(*http.Response,error){return f(r)}
func source(t *testing.T,raw string)Source{t.Helper();s,err:=ParseSource(raw);if err!=nil{t.Fatal(err)};return s}
func decode(t *testing.T,raw string)any{t.Helper();var data any;if err:=json.Unmarshal([]byte(raw),&data);err!=nil{t.Fatal(err)};return data}
func fixture(s Source,count int)Post{p:=Post{Source:s,Author:"Creator",Caption:"Field notes"};for i:=0;i<count;i++{p.Media=append(p.Media,Media{Kind:"image",URL:fmt.Sprintf("https://pbs.twimg.com/%d.jpg",i)})};return p}
func application(resolver Resolver)*App{return New(Config{PublicURL:"https://jade.example.com",Secret:strings.Repeat("s",64),MediaMode:"redirect"},resolver)}
func get(a *App,path string)*httptest.ResponseRecorder{r:=httptest.NewRequest("GET",path,nil);w:=httptest.NewRecorder();a.ServeHTTP(w,r);return w}

func TestSources(t *testing.T){
    for _,raw:=range []string{"https://www.instagram.com/p/AbCd123/?igsh=tracking","instagram.com/reel/AbCd123/","https://www.tiktok.com/@creator/photo/1234567890123456789","https://www.tiktok.com/@creator/video/1234567890123456789","https://vm.tiktok.com/AbC123/","https://twitter.com/creator/status/1234567890123456789/photo/2"}{s:=source(t,raw);round,err:=routeSource(s.SharePath());if err!=nil||round.URL!=s.URL{t.Fatalf("roundtrip: %+v %v",round,err)}}
    for _,raw:=range []string{"https://instagram.com.evil.test/p/abc/","https://instagram.com@127.0.0.1/p/abc/","http://127.0.0.1/","file:///etc/passwd","https://instagram.com:444/p/abc/","https://instagram.com/profile/","https://x.com/user","https://instagram.com/p/abc/ extra"}{if _,err:=ParseSource(raw);err==nil{t.Fatalf("accepted %q",raw)}}
}

func TestInstagramCarousel(t *testing.T){
    s:=source(t,"https://instagram.com/p/AbC123/")
    data:=decode(t,`{"data":{"xdt_shortcode_media":{"shortcode":"AbC123","owner":{"username":"archaeology"},"edge_media_to_caption":{"edges":[{"node":{"text":"Field notes"}}]},"edge_sidecar_to_children":{"edges":[{"node":{"is_video":false,"display_url":"https://scontent.cdninstagram.com/one.jpg"}},{"node":{"is_video":true,"video_url":"https://scontent.cdninstagram.com/two.mp4","display_url":"https://scontent.cdninstagram.com/two.jpg"}},{"node":{"is_video":false,"display_url":"https://scontent.cdninstagram.com/three.jpg"}}]}}}}`)
    p,err:=parseInstagram(data,s);if err!=nil{t.Fatal(err)}
    if len(p.Media)!=3||p.Media[0].Kind!="image"||p.Media[1].Kind!="video"||!strings.Contains(p.Media[2].URL,"three.jpg"){t.Fatalf("lost order: %+v",p)}
    if p.Author!="archaeology"||p.Caption!="Field notes"{t.Fatal(p)}
}

func TestInstagramPhotoOnlyAndIncomplete(t *testing.T){
    s:=source(t,"https://instagram.com/p/AbC123/")
    data:=decode(t,`{"items":[{"code":"AbC123","carousel_media_count":2,"carousel_media":[{"media_type":1,"image_versions2":{"candidates":[{"url":"https://a.fbcdn.net/1.jpg","width":900,"height":1200}]}},{"media_type":1,"image_versions2":{"candidates":[{"url":"https://a.fbcdn.net/2.jpg"}]}}]}]}`)
    p,err:=parseInstagram(data,s);if err!=nil||len(p.Media)!=2{t.Fatalf("photo-only: %+v %v",p,err)}
    node:=obj(arr(obj(data)["items"])[0]);node["carousel_media_count"]=float64(3)
    if _,err=parseInstagram(data,s);err==nil{t.Fatal("accepted incomplete carousel")}
}

func TestTikTokPhotoModeAndVideo(t *testing.T){
    s:=source(t,"https://tiktok.com/@creator/photo/1234567890123456789")
    data:=decode(t,`{"__DEFAULT_SCOPE__":{"webapp.video-detail":{"itemInfo":{"itemStruct":{"id":"1234567890123456789","author":{"uniqueId":"creator"},"desc":"Three slides","imagePost":{"images":[{"imageURL":{"urlList":["https://p16.tiktokcdn.com/1.jpeg"]}},{"imageURL":{"urlList":["https://p16.tiktokcdn.com/2.jpeg"]}},{"imageURL":{"urlList":["https://p16.tiktokcdn.com/3.jpeg"]}}]}}}}}}`)
    p,err:=parseTikTok(data,s);if err!=nil||len(p.Media)!=3||p.Media[2].Kind!="image"{t.Fatalf("photo mode: %+v %v",p,err)}
    node:=object{"id":s.ID,"author":object{"uniqueId":"creator"},"video":object{"playAddr":"https://v16.tiktokcdn.com/a.mp4","cover":"https://p16.tiktokcdn.com/cover.jpg"}}
    p,err=parseTikTok(node,s);if err!=nil||len(p.Media)!=1||p.Media[0].Kind!="video"{t.Fatalf("video: %+v %v",p,err)}
}

func TestTwitterMixedMedia(t *testing.T){
    s:=source(t,"https://x.com/creator/status/1234567890123456789")
    data:=decode(t,`{"id_str":"1234567890123456789","user":{"screen_name":"creator"},"text":"A mixed post","mediaDetails":[{"type":"photo","media_url_https":"https://pbs.twimg.com/1.jpg"},{"type":"video","media_url_https":"https://pbs.twimg.com/2.jpg","video_info":{"variants":[{"content_type":"application/x-mpegURL","url":"https://video.twimg.com/a.m3u8","bitrate":99999},{"content_type":"video/mp4","url":"https://video.twimg.com/low.mp4","bitrate":100},{"content_type":"video/mp4","url":"https://video.twimg.com/high.mp4","bitrate":200}]}}]}`)
    p,err:=parseTwitter(data,s);if err!=nil{t.Fatal(err)}
    if len(p.Media)!=2||p.Media[0].Kind!="image"||p.Media[1].Kind!="video"||!strings.HasSuffix(p.Media[1].URL,"high.mp4"){t.Fatalf("wrong media order/format: %+v",p)}
}

func TestTwitterMixedMedia(t *testing.T){
    s:=source(t,"https://x.com/creator/status/1234567890123456789")
    data:=decode(t,`{"id_str":"1234567890123456789","user":{"screen_name":"creator"},"text":"A mixed post","mediaDetails":[{"type":"photo","media_url_https":"https://pbs.twimg.com/1.jpg"},{"type":"video","media_url_https":"https://pbs.twimg.com/2.jpg","video_info":{"variants":[{"content_type":"application/x-mpegURL","url":"https://video.twimg.com/a.m3u8","bitrate":99999},{"content_type":"video/mp4","url":"https://video.twimg.com/low.mp4","bitrate":100},{"content_type":"video/mp4","url":"https://video.twimg.com/high.mp4","bitrate":200}]}},{"type":"photo","media_url_https":"https://pbs.twimg.com/3.jpg"}]}`)
    post,err:=parseTwitter(data,s);if err!=nil||len(post.Media)!=3||!strings.Contains(post.Media[1].URL,"high.mp4"){t.Fatalf("mixed media: %+v %v",post,err)}
}

func TestGalleryLimitsAndBoundaries(t *testing.T){
    s:=source(t,"https://x.com/creator/status/1234567890123456789");a:=application(nil);post:=fixture(s,23);links:=a.links(post)
    first,err:=a.gallery(post,links,a.cfg.PublicURL+s.SharePath(),1);if err!=nil{t.Fatal(err)}
    if first.End!=10||first.Pages!=3{t.Fatal(first)}
    previous:=0
    for page:=1;page<=first.Pages;page++{g,err:=a.gallery(post,links,a.cfg.PublicURL+s.SharePath(),page);if err!=nil{t.Fatal(err)};if g.Start!=previous+1||g.End-g.Start>=10{t.Fatal("bad boundary",g)};previous=g.End;b,_:=json.Marshal(g.Payload);if len(b)>3000{t.Fatal("oversized component JSON")}}
    if previous!=23{t.Fatal("lost slides",previous)}
    if _,err=a.gallery(post,links,a.cfg.PublicURL+s.SharePath(),4);err==nil{t.Fatal("accepted page beyond end")}
}

func TestHTTPEmbedsAndAssets(t *testing.T){
    s:=source(t,"https://x.com/creator/status/1234567890123456789");var calls atomic.Int32
    a:=application(resolverFunc(func(context.Context,Source)(Post,error){calls.Add(1);p:=fixture(s,3);p.Caption="</script><img src=x onerror=alert(1)>";return p,nil}))
    w:=get(a,"/api/link?url="+url.QueryEscape(s.URL));if w.Code!=200||calls.Load()!=0{t.Fatal(w.Code,w.Body.String())}
    w=get(a,s.SharePath());if w.Code!=200{t.Fatal(w.Code,w.Body.String())};body:=w.Body.String();match:=componentPattern.FindStringSubmatch(body)
    if len(match)!=2||strings.Contains(body,"<img src=x onerror=alert(1)>"){t.Fatal("missing component payload or unsafe HTML")};var payload any;if json.Unmarshal([]byte(match[1]),&payload)!=nil{t.Fatal("invalid embedded JSON",match[1])}
    for i,link:=range a.links(fixture(s,3)){u,_:=url.Parse(link.URL);response:=get(a,u.RequestURI());if response.Code!=307||!strings.Contains(response.Header().Get("Location"),fmt.Sprintf("/%d.jpg",i)){t.Fatal("wrong asset",response.Code)}}
    if calls.Load()!=1{t.Fatal("metadata not cached")}
    if w=get(a,"/a/forged.token/0.jpg");w.Code!=403{t.Fatal(w.Code)}
    if w=get(a,s.SharePath()+"?page=99");w.Code!=400{t.Fatal(w.Code)}
    if w=get(a,"/healthz");w.Code!=200{t.Fatal(w.Code)}
}

func TestResolveCacheCoalesces(t *testing.T){
    s:=source(t,"https://x.com/creator/status/1234567890123456789");var calls atomic.Int32
    a:=application(resolverFunc(func(context.Context,Source)(Post,error){calls.Add(1);time.Sleep(10*time.Millisecond);return fixture(s,2),nil}));var wg sync.WaitGroup
    for i:=0;i<20;i++{wg.Add(1);go func(){defer wg.Done();if _,err:=a.resolve(context.Background(),s);err!=nil{t.Error(err)}}()};wg.Wait();if calls.Load()!=1{t.Fatal("duplicate provider work",calls.Load())}
}

func TestNetworkPolicyAndTokenScope(t *testing.T){
    for _,raw:=range []string{"127.0.0.1","10.0.0.1","169.254.169.254","100.64.0.1","192.168.2.1","::1","::ffff:127.0.0.1","fc00::1","2001:db8::1"}{if publicIP(netip.MustParseAddr(raw)){t.Fatal("allowed",raw)}}
    if !publicIP(netip.MustParseAddr("8.8.8.8")){t.Fatal("public IP rejected")}
    for _,raw:=range []string{"https://pbs.twimg.com.evil.test/a.jpg","https://pbs.twimg.com@127.0.0.1/a.jpg","http://pbs.twimg.com/a.jpg","https://pbs.twimg.com:444/a.jpg"}{if mediaAllowed(raw,"x"){t.Fatal("allowed",raw)}}
    s:=source(t,"https://x.com/creator/status/1234567890123456789");a:=application(nil);token:=a.token(s)
    if result,err:=a.verify(token);err!=nil||result.URL!=s.URL{t.Fatal(result,err)}
    if _,err:=a.verify("x"+token);err==nil{t.Fatal("accepted forged token")}
}

func TestProviderHTTPAndEmbeddedJSON(t *testing.T){
    s:=source(t,"https://tiktok.com/@creator/photo/1234567890123456789")
    body:=`<script id="__UNIVERSAL_DATA_FOR_REHYDRATION__" type="application/json">{"itemInfo":{"itemStruct":{"id":"1234567890123456789","imagePost":{"images":[{"imageURL":{"urlList":["https://p16.tiktokcdn.com/1.jpeg"]}}]}}}}</script>`
    p:=Providers{Client:&http.Client{Transport:transportFunc(func(req *http.Request)(*http.Response,error){return &http.Response{StatusCode:200,Body:io.NopCloser(strings.NewReader(body)),Request:req,Header:make(http.Header)},nil})}}
    post,err:=p.Resolve(context.Background(),s);if err!=nil||len(post.Media)!=1{t.Fatalf("embedded JSON: %+v %v",post,err)}
    if _,_,err:=p.read(context.Background(),"http://169.254.169.254/",nil);err==nil{t.Fatal("metadata URL not blocked")}
}
