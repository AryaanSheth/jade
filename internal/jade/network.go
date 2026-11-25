package jade

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "net/netip"
    "net/url"
    "strings"
    "time"
)

var reserved = []string{"0.0.0.0/8","10.0.0.0/8","100.64.0.0/10","127.0.0.0/8","169.254.0.0/16","172.16.0.0/12","192.0.0.0/24","192.0.2.0/24","192.168.0.0/16","198.18.0.0/15","198.51.100.0/24","203.0.113.0/24","224.0.0.0/4","240.0.0.0/4","2001:db8::/32","fc00::/7","fe80::/10"}
func publicIP(ip netip.Addr) bool {
    ip=ip.Unmap()
    if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() { return false }
    for _, raw:= range reserved { if netip.MustParsePrefix(raw).Contains(ip) { return false } }
    return true
}

func hostIn(host string, domains ...string) bool { for _, d:=range domains { if host==d || strings.HasSuffix(host,"."+d) { return true } }; return false }
func mediaAllowed(raw,platform string) bool {
    u,err:=url.Parse(raw); if err!=nil || u.Scheme!="https" || u.User!=nil || u.Port()!="" || u.Fragment!="" || len(raw)>4096 { return false }
    h:=strings.ToLower(u.Hostname())
    switch platform {
    case "instagram":return hostIn(h,"cdninstagram.com","fbcdn.net")
    case "x":return hostIn(h,"twimg.com")
    case "tiktok":return hostIn(h,"tiktokcdn.com","tiktokcdn-us.com","tiktokcdn-eu.com","tiktokv.com","tiktokv.us","tiktok.com","byteoversea.com","ibytedtos.com","ibyteimg.com","muscdn.com","byteimg.com")
    }
    return false
}
func pageAllowed(raw string) bool {
    u,err:=url.Parse(raw); if err!=nil || u.Scheme!="https" || u.User!=nil || u.Port()!="" { return false }
    switch u.Hostname() { case "www.instagram.com","i.instagram.com","www.tiktok.com","m.tiktok.com","vm.tiktok.com","vt.tiktok.com","cdn.syndication.twimg.com":return true };return false
}

func safeClient() *http.Client {
    transport:= &http.Transport{Proxy:nil, MaxIdleConns:32, MaxIdleConnsPerHost:4, IdleConnTimeout:30*time.Second, TLSHandshakeTimeout:3*time.Second, ResponseHeaderTimeout:5*time.Second, DisableCompression:true}
    transport.DialContext=func(ctx context.Context,network,address string)(net.Conn,error){
        host,port,err:=net.SplitHostPort(address); if err!=nil{return nil,err}
        ips,err:=net.DefaultResolver.LookupNetIP(ctx,"ip",host);if err!=nil{return nil,err}
        if len(ips)==0{return nil,fmt.Errorf("host has no public address")}
        for _,ip:=range ips { if !publicIP(ip) { return nil,fmt.Errorf("private or reserved upstream address") } }
        var last error
        for _,ip:=range ips { conn,e:=(&net.Dialer{Timeout:3*time.Second}).DialContext(ctx,network,net.JoinHostPort(ip.String(),port));if e==nil{return conn,nil};last=e };return nil,last
    }
    return &http.Client{Transport:transport,Timeout:8*time.Second,CheckRedirect:func(req *http.Request,via []*http.Request)error{
        if len(via)>3 || !pageAllowed(req.URL.String()) { return fmt.Errorf("upstream redirect blocked") };return nil
    }}
}
