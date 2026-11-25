package jade

import (
    "context"
    "encoding/json"
    "fmt"
    "html"
    "io"
    "math/big"
    "net/http"
    "net/url"
    "regexp"
    "sort"
    "strings"
)

type Media struct { Kind string `json:"kind"`; URL string `json:"url"`; Poster string `json:"poster,omitempty"`; Width int `json:"width,omitempty"`; Height int `json:"height,omitempty"` }
type Post struct { Source Source `json:"source"`; Author string `json:"author"`; Caption string `json:"caption"`; Media []Media `json:"media"` }
type Resolver interface { Resolve(context.Context,Source)(Post,error) }
type Providers struct { Client *http.Client; InstagramDocID string }
type object = map[string]any
func obj(v any) object { x,_:=v.(map[string]any);return x }
func arr(v any) []any { x,_:=v.([]any);return x }
func text(v any) string { s,_:=v.(string);return s }
func number(v any) int { switch n:=v.(type){case float64: return int(n);case json.Number: i,_:=n.Int64();return int(i)};return 0 }
func first(v ...any) string { for _,x:= range v { if s:=text(x);s!="" { return s } };return "" }
func nested(m object, keys ...string) any { var value any=m;for _,key:=range keys { value=obj(value)[key] };return value }

func (p *Providers) read(ctx context.Context,raw string,form url.Values)([]byte,string,error){
    if !pageAllowed(raw){return nil,"",fmt.Errorf("unsupported upstream")}
    method:=http.MethodGet;var body io.Reader
    if form!=nil {method=http.MethodPost;body=strings.NewReader(form.Encode())}
    req,err:=http.NewRequestWithContext(ctx,method,raw,body);if err!=nil{return nil,"",err}
    req.Header.Set("User-Agent","Mozilla/5.0 (compatible; Jade/1.0; public post preview)")
    req.Header.Set("Accept","text/html,application/json")
    if form!=nil {req.Header.Set("Content-Type","application/x-www-form-urlencoded");req.Header.Set("X-IG-App-ID","936619743392459");req.Header.Set("X-ASBD-ID","359341")}
    response,err:=p.Client.Do(req);if err!=nil{return nil,"",err};defer response.Body.Close()
    if response.StatusCode!=200{return nil,"",fmt.Errorf("platform returned HTTP %d",response.StatusCode)}
    b,err:=io.ReadAll(io.LimitReader(response.Body,3*1024*1024+1));if err!=nil{return nil,"",err};if len(b)>3*1024*1024{return nil,"",fmt.Errorf("post metadata is too large")};return b,response.Request.URL.String(),nil
}

func (p *Providers) Resolve(ctx context.Context,s Source)(Post,error){
    if s.Short {_,raw,err:=p.read(ctx,s.URL,nil);if err!=nil{return Post{},err};expanded,err:=ParseSource(raw);if err!=nil||expanded.Short{return Post{},fmt.Errorf("use the full TikTok post link")};s=expanded}
    if s.Platform=="x" {
        b,_,err:=p.read(ctx,"https://cdn.syndication.twimg.com/tweet-result?id="+s.ID+"&lang=en&token=0",nil);if err!=nil{return Post{},err};var data any
        if json.Unmarshal(b,&data)!=nil{return Post{},fmt.Errorf("invalid platform response")};return parseTwitter(data,s)
    }
    if s.Platform=="instagram" {
        // The public GraphQL document ID can be updated without a code release.
        if p.InstagramDocID!="" {
            id:=shortcodeID(s.ID);vars,_:=json.Marshal(map[string]string{"media_id":id})
            b,_,err:=p.read(ctx,"https://www.instagram.com/graphql/query/",url.Values{"doc_id":{p.InstagramDocID},"variables":{string(vars)}})
            if err==nil {var data any;if json.Unmarshal(b,&data)==nil {if post,e:=parseInstagram(data,s);e==nil{return post,nil}}}
        }
        b,_,err:=p.read(ctx,s.URL+"embed/captioned/",nil);if err!=nil{return Post{},err};return parseDocuments(b,s)
    }
    b,_,err:=p.read(ctx,s.URL,nil);if err!=nil{return Post{},err};return parseDocuments(b,s)
}

func shortcodeID(code string)string{n:=new(big.Int);alphabet:="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";for _,r:=range code{i:=strings.IndexRune(alphabet,r);if i<0{return ""};n.Mul(n,big.NewInt(64));n.Add(n,big.NewInt(int64(i)))};return n.String()}
var scripts=regexp.MustCompile(`(?is)<script\b[^>]*>(.*?)</script\s*>`)
func parseDocuments(b []byte,s Source)(Post,error){
    documents:=[]any{};var v any
    if json.Unmarshal(b,&v)==nil {documents=append(documents,v)}
    for _,match:=range scripts.FindAllSubmatch(b,256) {
        raw:=strings.TrimSpace(string(match[1]));if json.Unmarshal([]byte(raw),&v)==nil{documents=append(documents,v);continue}
        documents=append(documents,embeddedJSONDocuments(raw,"\"contextJSON\":")...)
        if at:=strings.Index(raw,"=");at>=0 {raw=strings.TrimSuffix(strings.TrimSpace(raw[at+1:]),";");if json.Unmarshal([]byte(raw),&v)==nil{documents=append(documents,v)}}
    }
    for _,d:=range documents { var post Post;var err error;if s.Platform=="instagram" {post,err=parseInstagram(d,s)}else{post,err=parseTikTok(d,s)};if err==nil{return post,nil} }
    return Post{},fmt.Errorf("the platform did not expose this post's complete media; it may require login or block server requests")
}

// Instagram embeds store the post JSON inside an escaped contextJSON string.
func embeddedJSONDocuments(raw,marker string)[]any{
    documents:=[]any{}
    for offset:=0;; {at:=strings.Index(raw[offset:],marker);if at<0{return documents};at+=offset+len(marker);decoder:=json.NewDecoder(strings.NewReader(raw[at:]));var encoded string;if decoder.Decode(&encoded)==nil{var document any;if json.Unmarshal([]byte(encoded),&document)==nil{documents=append(documents,document)}};offset=at}
}

// Walk metadata, never arbitrary linked pages. Limit depth to bound nested input.
func find(v any,depth int,predicate func(object)bool)object{
    if depth>35{return nil}
    if m:=obj(v);m!=nil {if predicate(m){return m};keys:=make([]string,0,len(m));for key:=range m{keys=append(keys,key)};sort.Strings(keys);for _,key:=range keys {if found:=find(m[key],depth+1,predicate);found!=nil{return found}}}
    for _,x:=range arr(v){if found:=find(x,depth+1,predicate);found!=nil{return found}};return nil
}

func parseInstagram(data any,s Source)(Post,error){
    node:=find(data,0,func(m object)bool{
        code:=first(m["shortcode"],m["code"])
        return code==s.ID && (m["display_url"]!=nil||m["image_versions2"]!=nil||m["carousel_media"]!=nil||m["edge_sidecar_to_children"]!=nil)
    })
    if node==nil{return Post{},fmt.Errorf("post metadata missing")}
    if owner:=obj(node["owner"]);owner["is_private"]==true{return Post{},fmt.Errorf("private post")}
    post:=Post{Source:s,Author:first(nested(node,"owner","username"),nested(node,"user","username"))}
    post.Caption=first(nested(node,"caption","text"),node["caption"])
    if edges:=arr(nested(node,"edge_media_to_caption","edges"));len(edges)>0{post.Caption=first(nested(obj(edges[0]),"node","text"),post.Caption)}
    children:=arr(node["carousel_media"])
    if edges:=arr(nested(node,"edge_sidecar_to_children","edges"));len(edges)>0 {children=nil;for _,e:=range edges{children=append(children,obj(e)["node"])}}
    if len(children)==0 {children=[]any{node}}
    if count:=number(node["carousel_media_count"]);count>len(children){return Post{},fmt.Errorf("incomplete carousel metadata")}
    for _,child:=range children {
        m:=obj(child);media:=Media{Kind:"image",Width:number(nested(m,"dimensions","width")),Height:number(nested(m,"dimensions","height"))}
        media.Poster=first(m["display_url"],m["thumbnail_src"])
        for _,c:=range arr(nested(m,"image_versions2","candidates")){candidate:=obj(c);if media.Poster=="" {media.Poster=text(candidate["url"]);media.Width=number(candidate["width"]);media.Height=number(candidate["height"]);break}}
        if m["is_video"]==true||number(m["media_type"])==2||m["video_url"]!=nil||len(arr(m["video_versions"]))>0 {
            media.Kind="video";media.URL=text(m["video_url"])
            for _,c:=range arr(m["video_versions"]){if media.URL==""{media.URL=text(obj(c)["url"])}}
        }else{media.URL=media.Poster}
        post.Media=append(post.Media,media)
    }
    return validatePost(post)
}

func urlList(v any)string {m:=obj(v);for _,key:=range []string{"urlList","url_list","UrlList"}{for _,u:=range arr(m[key]){if text(u)!=""{return text(u)}}};return first(m["url"],v)}
func tiktokVideoURL(video object)string{
    best:="";bitrate:=-1
    for _,item:=range arr(video["bitrateInfo"]){variant:=obj(item);if text(variant["CodecType"])!="h264"{continue};for _,candidate:=range arr(nested(variant,"PlayAddr","UrlList")){raw:=text(candidate);u,err:=url.Parse(raw);if err==nil&&u.Hostname()=="www.tiktok.com"&&u.Path=="/aweme/v1/play/"&&number(variant["Bitrate"])>bitrate{best=raw;bitrate=number(variant["Bitrate"])}}}
    if best!=""{return best};raw:=urlList(video["playAddr"]);if raw==""{raw=urlList(video["downloadAddr"])};return raw
}
func parseTikTok(data any,s Source)(Post,error){
    node:=find(data,0,func(m object)bool{return first(m["id"],m["aweme_id"])==s.ID && (m["video"]!=nil||m["imagePost"]!=nil||m["image_post_info"]!=nil)})
    if node==nil{return Post{},fmt.Errorf("post metadata missing")}
    post:=Post{Source:s,Author:first(nested(node,"author","uniqueId"),nested(node,"author","unique_id")),Caption:first(node["desc"],node["description"])}
    images:=arr(nested(node,"imagePost","images"));if images==nil {images=arr(nested(node,"image_post_info","images"))}
    if len(images)>0 {
        for _,image:=range images {m:=obj(image);raw:=urlList(m["imageURL"]);if raw==""{raw=urlList(m["display_image"])};post.Media=append(post.Media,Media{Kind:"image",URL:raw,Width:number(m["imageWidth"]),Height:number(m["imageHeight"])})}
    }else{
        v:=obj(node["video"]);raw:=tiktokVideoURL(v)
        post.Media=[]Media{{Kind:"video",URL:raw,Poster:urlList(v["cover"]),Width:number(v["width"]),Height:number(v["height"])}}
    }
    return validatePost(post)
}

func parseTwitter(data any,s Source)(Post,error){
    node:=obj(data);if first(node["id_str"],node["id"])!=s.ID{return Post{},fmt.Errorf("post unavailable")}
    post:=Post{Source:s,Author:first(nested(node,"user","screen_name"),nested(node,"user","name")),Caption:first(node["text"],node["full_text"])}
    details:=arr(node["mediaDetails"])
    if details==nil {details=arr(nested(node,"extended_entities","media"))}
    for _,item:=range details{
        m:=obj(item);media:=Media{Kind:"image",URL:first(m["media_url_https"],m["media_url"]),Width:number(nested(m,"original_info","width")),Height:number(nested(m,"original_info","height"))}
        if m["type"]=="video"||m["type"]=="animated_gif" {
            media.Kind="video";media.Poster=media.URL;media.URL="";best:=-1
            for _,variant:=range arr(nested(m,"video_info","variants")){v:=obj(variant);raw:=first(v["url"],v["src"]);if (v["content_type"]=="video/mp4"||v["type"]=="video/mp4")&&number(v["bitrate"])>=best{best=number(v["bitrate"]);media.URL=raw}}
        }
        post.Media=append(post.Media,media)
    }
    // Older syndication responses publish photo arrays without mediaDetails.
    if details==nil {for _,photo:=range arr(node["photos"]){m:=obj(photo);post.Media=append(post.Media,Media{Kind:"image",URL:text(m["url"]),Width:number(m["width"]),Height:number(m["height"])})}}
    return validatePost(post)
}

func validatePost(post Post)(Post,error){
    if len(post.Media)==0{return Post{},fmt.Errorf("this post has no supported media")};if len(post.Media)>100{return Post{},fmt.Errorf("this post exceeds the 100-item service limit")}
    for i:=range post.Media {m:=&post.Media[i];m.URL=html.UnescapeString(m.URL);m.Poster=html.UnescapeString(m.Poster);if !mediaAllowed(m.URL,post.Source.Platform){return Post{},fmt.Errorf("slide %d has no accessible media URL",i+1)};if !mediaAllowed(m.Poster,post.Source.Platform){m.Poster=""};if m.Width<0||m.Width>16384{m.Width=0};if m.Height<0||m.Height>16384{m.Height=0}}
    post.Caption=truncate(post.Caption,1800);post.Author=truncate(post.Author,100);return post,nil
}
func truncate(s string,n int)string{r:=[]rune(s);if len(r)>n{return string(r[:n])};return s}
