Hikarinagi Public API
Hikarinagi 公开数据 API。所有端点要求携带 client_credentials 访问令牌，默认 throttle 60 次/分钟/应用。

获取访问令牌
在 开发者控制台 创建应用后，用 client_id 与 client_secret 通过 Basic 认证请求令牌，令牌有效期 1 小时；scope 为本次申请的权限集合，不能超出应用已被授予的范围。


curl -X POST "https://id.hikarinagi.org/oidc/token" \
  -u "$CLIENT_ID:$CLIENT_SECRET" \
  -d "grant_type=client_credentials&scope=$SCOPES"
响应格式
所有响应都包裹在统一信封中，文档里各端点描述的是信封内 data 字段的结构。


{
  "success": true,
  "data": { … },
  "request_id": "req-x",
  "timestamp": "2026-07-10T00:00:00.000Z"
}

Galgame 详情
GET
/api/v3/open/galgames/{id}

查询参数
id*
number
响应数据
adv_type*
string | null
aliases*
string[]
covers*
OpenCoverDto[]
height*
number | null
sexual*
number
色情内容分级，0 为安全
url*
string
完整图片 URL
violence*
number
暴力内容分级，0 为安全
votes*
number
封面得票数
width*
number | null
created_at*
string
dev_status*
"RELEASED" | "IN_DEVELOPMENT" | "CANCELLED" | null
engine*
string | null
homepage*
string | null
id*
number
images*
OpenMediaDto[]
height*
number | null
sexual*
number
色情内容分级，0 为安全
url*
string
完整图片 URL
violence*
number
暴力内容分级，0 为安全
width*
number | null
nsfw*
boolean
origin_intro*
string | null
origin_lang*
string | null
origin_title*
string
platforms*
string[]
prices*
OpenGalgamePriceDto[]
amount*
number | null
currency*
string | null
tax_included*
boolean | null
version*
string | null
release_date*
string | null
release_date_tbd*
boolean
release_date_tbd_note*
string
revised_at*
string | null
tags*
OpenTagDto[]
likes*
number
name*
string
trans_intro*
string | null
trans_title*
string | null
updated_at*
string

curl "https://www.hikarinagi.org/api/v3/open/galgames/1" \
  -H "Authorization: Bearer $ACCESS_TOKEN"