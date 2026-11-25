package jade

import (
    "encoding/json"
    "fmt"
    "strconv"
    "strings"
)

type Gallery struct { Page int `json:"page"`; Pages int `json:"pages"`; Start int `json:"start"`; End int `json:"end"`; Total int `json:"total"`; ShareURL string `json:"share_url"`; Previous string `json:"previous,omitempty"`; Next string `json:"next,omitempty"`; Payload map[string]any `json:"payload"` }
func (a *App) gallery(post Post,links []Media,share string,page int)(Gallery,error){
    // Every page must fit using the same size, or boundaries could skip slides.
    for size:=10;size>=1;size-- {
        pages:=(len(links)+size-1)/size
        all:=make([]Gallery,pages);fits:=true
        for current:=1;current<=pages;current++ {
            g:=galleryPage(post,links,share,current,size)
            encoded,err:=json.Marshal(g.Payload);if err!=nil{return Gallery{},err}
            if len(encoded)>3000{fits=false;break};all[current-1]=g
        }
        if fits {if page<1||page>pages{return Gallery{},fmt.Errorf("gallery page does not exist")};return all[page-1],nil}
    }
    return Gallery{},fmt.Errorf("gallery cannot fit Discord's payload limit")
}

func galleryPage(post Post,links []Media,share string,page,size int)Gallery{
    pages:=(len(links)+size-1)/size;start:=(page-1)*size;end:=start+size;if end>len(links){end=len(links)}
    current:=share;if page>1{current+="?page="+strconv.Itoa(page)}
    items:=[]any{}
    for i:=start;i<end;i++ {items=append(items,map[string]any{"media":map[string]string{"url":links[i].URL},"description":"Slide "+strconv.Itoa(i+1)})}
    platform := map[string]string{"instagram": "Instagram", "tiktok": "TikTok", "x": "X"}[post.Source.Platform]
    if platform == "" {
        platform = "Jade"
    }
    name := truncate(strings.Join(strings.Fields(post.Author), " "), 80)
    heading := "**" + platform + "**"
    if name != "" {
        escape := strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "~", "\\~", "`", "\\`", "|", "\\|", "<", "‹")
        heading = "**" + escape.Replace(name) + "** · " + platform
    }
    if pages > 1 {
        heading += fmt.Sprintf(" · Slides %d–%d of %d", start+1, end, len(links))
    }
    caption := strings.Join(strings.Fields(post.Caption), " ")
    if caption != "" {
        if shortened := truncate(caption, 179); shortened != caption {
            caption = strings.TrimSpace(shortened) + "…"
        }
        heading += "\n" + caption
    }
    components := []any{
        map[string]any{"type": 10, "content": heading},
        map[string]any{"type": 12, "items": items},
    }
    label, destination := "Open original", post.Source.URL
    if pages > 1 {
        label, destination = "View full post", share
    }
    buttons := []any{map[string]any{"type": 2, "style": 5, "label": label, "url": destination}}
    if page>1{buttons=append(buttons,map[string]any{"type":2,"style":5,"label":"Previous set","url":share+"?page="+strconv.Itoa(page-1)})}
    if page<pages{buttons=append(buttons,map[string]any{"type":2,"style":5,"label":"Next set","url":share+"?page="+strconv.Itoa(page+1)})}
    components=append(components,map[string]any{"type":1,"components":buttons})
    payload:=map[string]any{"component":map[string]any{"type":17,"accent_color":6205312,"components":components}}
    g:=Gallery{Page:page,Pages:pages,Start:start+1,End:end,Total:len(links),ShareURL:current,Payload:payload}
    if page>1{g.Previous=share+"?page="+strconv.Itoa(page-1)}
    if page<pages{g.Next=share+"?page="+strconv.Itoa(page+1)}
    return g
}
