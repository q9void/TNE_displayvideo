// Package openrtb provides OpenRTB 2.6 data models
package openrtb

import "encoding/json"

// InventoryType represents the type of inventory in a bid request
type InventoryType int

const (
	InventoryTypeSite InventoryType = iota
	InventoryTypeApp
	InventoryTypeDOOH
)

// BidRequest represents an OpenRTB 2.6 bid request
type BidRequest struct {
	ID     string          `json:"id"`
	Imp    []Imp           `json:"imp"`
	Site   *Site           `json:"site,omitempty"`
	App    *App            `json:"app,omitempty"`
	DOOH   *DOOH           `json:"dooh,omitempty"` // 2.6: Digital Out-of-Home
	Device *Device         `json:"device,omitempty"`
	User   *User           `json:"user,omitempty"`
	Test   int             `json:"test,omitempty"`
	AT     int             `json:"at,omitempty"`     // Auction type: 1=first price, 2=second price
	TMax   int             `json:"tmax,omitempty"`   // Max time in ms for bid response
	WSeat  []string        `json:"wseat,omitempty"`  // Allowed buyer seats
	BSeat  []string        `json:"bseat,omitempty"`  // Blocked buyer seats
	AllImp int             `json:"allimps,omitempty"`
	Cur    []string        `json:"cur,omitempty"`    // Allowed currencies
	WLang  []string        `json:"wlang,omitempty"`  // Allowed languages (ISO-639-1-Alpha-2)
	WLangB []string        `json:"wlangb,omitempty"` // 2.6: Allowed languages (BCP-47)
	BCat   []string        `json:"bcat,omitempty"`   // Blocked categories
	CatTax int             `json:"cattax,omitempty"` // 2.6: Category taxonomy (default 1 = IAB Content Category Taxonomy 1.0)
	BAdv   []string        `json:"badv,omitempty"`   // Blocked advertisers
	BApp   []string        `json:"bapp,omitempty"`   // Blocked apps
	Source *Source          `json:"source,omitempty"`
	Regs   *Regs           `json:"regs,omitempty"`
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// InventoryType returns the inventory type of the bid request
func (br *BidRequest) InventoryType() InventoryType {
	if br.DOOH != nil {
		return InventoryTypeDOOH
	}
	if br.App != nil {
		return InventoryTypeApp
	}
	return InventoryTypeSite
}

// IsCTV returns true if this request represents Connected TV inventory
func (br *BidRequest) IsCTV() bool {
	if br.Device != nil {
		// DeviceType 3 = Connected TV, 7 = Set Top Box
		if br.Device.DeviceType == 3 || br.Device.DeviceType == 7 {
			return true
		}
	}
	// App with CTV-like bundle patterns or CTV device
	if br.App != nil && br.Device != nil && br.Device.DeviceType == 3 {
		return true
	}
	return false
}

// HasAdPods returns true if any impression contains ad pod signals
func (br *BidRequest) HasAdPods() bool {
	for i := range br.Imp {
		if br.Imp[i].Video != nil && br.Imp[i].Video.PodID != "" {
			return true
		}
		if br.Imp[i].Audio != nil && br.Imp[i].Audio.PodID != "" {
			return true
		}
	}
	return false
}

// Imp represents an impression object
type Imp struct {
	ID                string          `json:"id"`
	Metric            []Metric        `json:"metric,omitempty"`
	Banner            *Banner         `json:"banner,omitempty"`
	Video             *Video          `json:"video,omitempty"`
	Audio             *Audio          `json:"audio,omitempty"`
	Native            *Native         `json:"native,omitempty"`
	PMP               *PMP            `json:"pmp,omitempty"`
	DisplayManager    string          `json:"displaymanager,omitempty"`
	DisplayManagerVer string          `json:"displaymanagerver,omitempty"`
	Instl             int             `json:"instl,omitempty"` // Interstitial flag
	TagID             string          `json:"tagid,omitempty"`
	BidFloor          float64         `json:"bidfloor,omitempty"`
	BidFloorCur       string          `json:"bidfloorcur,omitempty"`
	ClickBrowser      int             `json:"clickbrowser,omitempty"`
	Secure            *int            `json:"secure,omitempty"`
	IframeBuster      []string        `json:"iframebuster,omitempty"`
	Exp               int             `json:"exp,omitempty"`
	SSAI              int             `json:"ssai,omitempty"`   // 2.6: Server-side ad insertion status
	Rwdd              int             `json:"rwdd,omitempty"`   // 2.6: Rewarded inventory flag
	Qty               *Qty            `json:"qty,omitempty"`    // 2.6: Quantity/multiplier (DOOH)
	DT               float64         `json:"dt,omitempty"`     // 2.6: Timestamp when imp becomes available
	Refresh           *RefSettings    `json:"refresh,omitempty"` // 2.6: Refresh settings for auto-refreshing placements
	Ext               json.RawMessage `json:"ext,omitempty"`
}

// Qty represents the quantity/multiplier object for impressions (2.6)
// Used primarily in DOOH where one ad play may be viewed by multiple people
type Qty struct {
	Multiplier float64         `json:"multiplier,omitempty"` // Impression multiplier
	SourceType int             `json:"sourcetype,omitempty"` // Source of quantity measurement
	Vendor     string          `json:"vendor,omitempty"`     // Vendor providing quantity data
	Ext        json.RawMessage `json:"ext,omitempty"`
}

// RefSettings represents auto-refresh settings (2.6)
type RefSettings struct {
	RefSettings []RefInfo       `json:"refsettings,omitempty"` // Array of refresh info objects
	Ext         json.RawMessage `json:"ext,omitempty"`
}

// RefInfo represents individual refresh info (2.6)
type RefInfo struct {
	RefType int             `json:"reftype,omitempty"` // Type of refresh: 0=unknown, 1=manual, 2=auto
	MinInt  int             `json:"minint,omitempty"`  // Minimum refresh interval in seconds
	Ext     json.RawMessage `json:"ext,omitempty"`
}

// Banner represents a banner impression
type Banner struct {
	Format   []Format        `json:"format,omitempty"`
	W        int             `json:"w,omitempty"`
	H        int             `json:"h,omitempty"`
	WMax     int             `json:"wmax,omitempty"` // Deprecated
	HMax     int             `json:"hmax,omitempty"` // Deprecated
	WMin     int             `json:"wmin,omitempty"` // Deprecated
	HMin     int             `json:"hmin,omitempty"` // Deprecated
	BType    []int           `json:"btype,omitempty"` // Blocked banner types
	BAttr    []int           `json:"battr,omitempty"` // Blocked creative attributes
	Pos      int             `json:"pos,omitempty"`   // Ad position
	Mimes    []string        `json:"mimes,omitempty"`
	TopFrame int             `json:"topframe,omitempty"`
	ExpDir   []int           `json:"expdir,omitempty"` // Expandable directions
	API      []int           `json:"api,omitempty"`
	ID       string          `json:"id,omitempty"`
	VCM      int             `json:"vcm,omitempty"`
	Ext      json.RawMessage `json:"ext,omitempty"`
}

// Format represents size format
type Format struct {
	W      int             `json:"w,omitempty"`
	H      int             `json:"h,omitempty"`
	WRatio int             `json:"wratio,omitempty"`
	HRatio int             `json:"hratio,omitempty"`
	WMin   int             `json:"wmin,omitempty"`
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// Video represents a video impression
type Video struct {
	Mimes          []string        `json:"mimes,omitempty"`
	MinDuration    int             `json:"minduration,omitempty"`
	MaxDuration    int             `json:"maxduration,omitempty"`
	Protocols      []int           `json:"protocols,omitempty"`
	Protocol       int             `json:"protocol,omitempty"` // Deprecated
	W              int             `json:"w,omitempty"`
	H              int             `json:"h,omitempty"`
	StartDelay     *int            `json:"startdelay,omitempty"`
	Placement      int             `json:"placement,omitempty"`  // Deprecated in 2.6, use Plcmt
	Plcmt          int             `json:"plcmt,omitempty"`      // 2.6: Video placement type (replaces placement)
	Linearity      int             `json:"linearity,omitempty"`
	Skip           *int            `json:"skip,omitempty"`
	SkipMin        int             `json:"skipmin,omitempty"`
	SkipAfter      int             `json:"skipafter,omitempty"`
	Sequence       int             `json:"sequence,omitempty"`   // Deprecated in 2.6, use podid/podseq
	BAttr          []int           `json:"battr,omitempty"`
	MaxExtended    int             `json:"maxextended,omitempty"`
	MinBitrate     int             `json:"minbitrate,omitempty"`
	MaxBitrate     int             `json:"maxbitrate,omitempty"`
	BoxingAllowed  int             `json:"boxingallowed,omitempty"`
	PlaybackMethod []int           `json:"playbackmethod,omitempty"`
	PlaybackEnd    int             `json:"playbackend,omitempty"`
	Delivery       []int           `json:"delivery,omitempty"`
	Pos            int             `json:"pos,omitempty"`
	CompanionAd    []Banner        `json:"companionad,omitempty"`
	API            []int           `json:"api,omitempty"`
	CompanionType  []int           `json:"companiontype,omitempty"`
	// 2.6: Ad pod fields
	MaxSeq       int             `json:"maxseq,omitempty"`       // Max number of ads in a pod
	PodDur       int             `json:"poddur,omitempty"`       // Pod duration in seconds
	PodID        string          `json:"podid,omitempty"`        // Pod identifier
	PodSeq       int             `json:"podseq,omitempty"`       // Pod sequence position in content
	RqdDurs      []int           `json:"rqddurs,omitempty"`      // Required exact durations (Live TV)
	SlotInPod    int             `json:"slotinpod,omitempty"`    // Slot position within pod
	MinCPMPerSec float64         `json:"mincpmpersec,omitempty"` // Minimum CPM per second for dynamic pods
	DurFloors    []DurFloor      `json:"durfloors,omitempty"`    // 2.6: Floor prices by duration
	Ext          json.RawMessage `json:"ext,omitempty"`
}

// Audio represents an audio impression
type Audio struct {
	Mimes         []string        `json:"mimes,omitempty"`
	MinDuration   int             `json:"minduration,omitempty"`
	MaxDuration   int             `json:"maxduration,omitempty"`
	Protocols     []int           `json:"protocols,omitempty"`
	StartDelay    *int            `json:"startdelay,omitempty"`
	Sequence      int             `json:"sequence,omitempty"` // Deprecated in 2.6
	BAttr         []int           `json:"battr,omitempty"`
	MaxExtended   int             `json:"maxextended,omitempty"`
	MinBitrate    int             `json:"minbitrate,omitempty"`
	MaxBitrate    int             `json:"maxbitrate,omitempty"`
	Delivery      []int           `json:"delivery,omitempty"`
	CompanionAd   []Banner        `json:"companionad,omitempty"`
	API           []int           `json:"api,omitempty"`
	CompanionType []int           `json:"companiontype,omitempty"`
	MaxSeq        int             `json:"maxseq,omitempty"`
	Feed          int             `json:"feed,omitempty"`
	Stitched      int             `json:"stitched,omitempty"`
	NVol          int             `json:"nvol,omitempty"`
	// 2.6: Ad pod fields
	PodDur       int             `json:"poddur,omitempty"`
	PodID        string          `json:"podid,omitempty"`
	PodSeq       int             `json:"podseq,omitempty"`
	RqdDurs      []int           `json:"rqddurs,omitempty"`
	SlotInPod    int             `json:"slotinpod,omitempty"`
	MinCPMPerSec float64         `json:"mincpmpersec,omitempty"`
	DurFloors    []DurFloor      `json:"durfloors,omitempty"`
	Ext          json.RawMessage `json:"ext,omitempty"`
}

// DurFloor represents a floor price for a specific creative duration (2.6)
type DurFloor struct {
	MinDur int     `json:"mindur,omitempty"` // Minimum duration in seconds
	MaxDur int     `json:"maxdur,omitempty"` // Maximum duration in seconds
	BidFloor float64 `json:"bidfloor,omitempty"` // Floor price for this duration range
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// Native represents a native impression
type Native struct {
	Request string          `json:"request,omitempty"`
	Ver     string          `json:"ver,omitempty"`
	API     []int           `json:"api,omitempty"`
	BAttr   []int           `json:"battr,omitempty"`
	Ext     json.RawMessage `json:"ext,omitempty"`
}

// Metric represents a metric object
type Metric struct {
	Type   string          `json:"type,omitempty"`
	Value  float64         `json:"value,omitempty"`
	Vendor string          `json:"vendor,omitempty"`
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// PMP represents a private marketplace
type PMP struct {
	PrivateAuction int             `json:"private_auction,omitempty"`
	Deals          []Deal          `json:"deals,omitempty"`
	Ext            json.RawMessage `json:"ext,omitempty"`
}

// Deal represents a deal object
type Deal struct {
	ID          string          `json:"id"`
	BidFloor    float64         `json:"bidfloor,omitempty"`
	BidFloorCur string          `json:"bidfloorcur,omitempty"`
	AT          int             `json:"at,omitempty"`
	WSeat       []string        `json:"wseat,omitempty"`
	WADomain    []string        `json:"wadomain,omitempty"`
	Ext         json.RawMessage `json:"ext,omitempty"`
}

// Site represents a website
type Site struct {
	ID                   string          `json:"id,omitempty"`
	Name                 string          `json:"name,omitempty"`
	Domain               string          `json:"domain,omitempty"`
	Cat                  []string        `json:"cat,omitempty"`
	SectionCat           []string        `json:"sectioncat,omitempty"`
	PageCat              []string        `json:"pagecat,omitempty"`
	Page                 string          `json:"page,omitempty"`
	Ref                  string          `json:"ref,omitempty"`
	Search               string          `json:"search,omitempty"`
	Mobile               int             `json:"mobile,omitempty"`
	PrivacyPolicy        int             `json:"privacypolicy,omitempty"`
	Publisher            *Publisher      `json:"publisher,omitempty"`
	Content              *Content        `json:"content,omitempty"`
	Keywords             string          `json:"keywords,omitempty"`
	KwArray              []string        `json:"kwarray,omitempty"`              // 2.6: Array of keywords (mutually exclusive with Keywords)
	CatTax               int             `json:"cattax,omitempty"`               // 2.6: Category taxonomy
	InventoryPartnerDomain string       `json:"inventorypartnerdomain,omitempty"` // 2.6: IPD for inventory sharing
	Ext                  json.RawMessage `json:"ext,omitempty"`
}

// App represents a mobile application
type App struct {
	ID                   string          `json:"id,omitempty"`
	Name                 string          `json:"name,omitempty"`
	Bundle               string          `json:"bundle,omitempty"`
	Domain               string          `json:"domain,omitempty"`
	StoreURL             string          `json:"storeurl,omitempty"`
	Cat                  []string        `json:"cat,omitempty"`
	SectionCat           []string        `json:"sectioncat,omitempty"`
	PageCat              []string        `json:"pagecat,omitempty"`
	Ver                  string          `json:"ver,omitempty"`
	PrivacyPolicy        int             `json:"privacypolicy,omitempty"`
	Paid                 int             `json:"paid,omitempty"`
	Publisher            *Publisher      `json:"publisher,omitempty"`
	Content              *Content        `json:"content,omitempty"`
	Keywords             string          `json:"keywords,omitempty"`
	KwArray              []string        `json:"kwarray,omitempty"`              // 2.6: Array of keywords
	CatTax               int             `json:"cattax,omitempty"`               // 2.6: Category taxonomy
	InventoryPartnerDomain string       `json:"inventorypartnerdomain,omitempty"` // 2.6: IPD for inventory sharing
	Ext                  json.RawMessage `json:"ext,omitempty"`
}

// DOOH represents a Digital Out-of-Home object (2.6)
// Mutually exclusive with Site and App
type DOOH struct {
	ID            string          `json:"id,omitempty"`
	Name          string          `json:"name,omitempty"`
	VenueType     []string        `json:"venuetype,omitempty"`   // Venue type codes
	VenueTypeTax  int             `json:"venuetypetax,omitempty"` // Venue type taxonomy
	Publisher     *Publisher      `json:"publisher,omitempty"`
	Domain        string          `json:"domain,omitempty"`
	Keywords      string          `json:"keywords,omitempty"`
	KwArray       []string        `json:"kwarray,omitempty"` // 2.6: Array of keywords
	Content       *Content        `json:"content,omitempty"`
	Ext           json.RawMessage `json:"ext,omitempty"`
}

// Publisher represents a publisher
type Publisher struct {
	ID     string          `json:"id,omitempty"`
	Name   string          `json:"name,omitempty"`
	Cat    []string        `json:"cat,omitempty"`
	CatTax int             `json:"cattax,omitempty"` // 2.6: Category taxonomy
	Domain string          `json:"domain,omitempty"`
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// Content represents content information
type Content struct {
	ID                 string          `json:"id,omitempty"`
	Episode            int             `json:"episode,omitempty"`
	Title              string          `json:"title,omitempty"`
	Series             string          `json:"series,omitempty"`
	Season             string          `json:"season,omitempty"`
	Artist             string          `json:"artist,omitempty"`
	Genre              string          `json:"genre,omitempty"`
	Album              string          `json:"album,omitempty"`
	ISRC               string          `json:"isrc,omitempty"`
	Producer           *Producer       `json:"producer,omitempty"`
	URL                string          `json:"url,omitempty"`
	Cat                []string        `json:"cat,omitempty"`
	CatTax             int             `json:"cattax,omitempty"` // 2.6: Category taxonomy
	ProdQ              int             `json:"prodq,omitempty"`
	VideoQuality       int             `json:"videoquality,omitempty"` // Deprecated
	Context            int             `json:"context,omitempty"`
	ContentRating      string          `json:"contentrating,omitempty"`
	UserRating         string          `json:"userrating,omitempty"`
	QAGMediaRating     int             `json:"qagmediarating,omitempty"`
	Keywords           string          `json:"keywords,omitempty"`
	KwArray            []string        `json:"kwarray,omitempty"` // 2.6: Array of keywords
	LiveStream         int             `json:"livestream,omitempty"`
	SourceRelationship int             `json:"sourcerelationship,omitempty"`
	Len                int             `json:"len,omitempty"`
	Language           string          `json:"language,omitempty"`
	LangB              string          `json:"langb,omitempty"` // 2.6: BCP-47 language
	Embeddable         int             `json:"embeddable,omitempty"`
	Data               []Data          `json:"data,omitempty"`
	Network            *Network        `json:"network,omitempty"` // 2.6: Content network (CTV)
	Channel            *Channel        `json:"channel,omitempty"` // 2.6: Content channel (CTV)
	Ext                json.RawMessage `json:"ext,omitempty"`
}

// Network represents a content network (2.6)
// e.g. a content licensor/owner like A+E Networks
type Network struct {
	ID     string          `json:"id,omitempty"`
	Name   string          `json:"name,omitempty"`
	Domain string          `json:"domain,omitempty"`
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// Channel represents a content channel (2.6)
// e.g. The HISTORY Channel, Lifetime
type Channel struct {
	ID     string          `json:"id,omitempty"`
	Name   string          `json:"name,omitempty"`
	Domain string          `json:"domain,omitempty"`
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// Producer represents a content producer
type Producer struct {
	ID     string          `json:"id,omitempty"`
	Name   string          `json:"name,omitempty"`
	Cat    []string        `json:"cat,omitempty"`
	Domain string          `json:"domain,omitempty"`
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// Device represents a user device
type Device struct {
	UA             string          `json:"ua,omitempty"`
	SUA            *UserAgent      `json:"sua,omitempty"` // 2.6: Structured User-Agent
	Geo            *Geo            `json:"geo,omitempty"`
	DNT            *int            `json:"dnt,omitempty"`
	Lmt            *int            `json:"lmt,omitempty"`
	IP             string          `json:"ip,omitempty"`
	IPv6           string          `json:"ipv6,omitempty"`
	DeviceType     int             `json:"devicetype,omitempty"`
	Make           string          `json:"make,omitempty"`
	Model          string          `json:"model,omitempty"`
	OS             string          `json:"os,omitempty"`
	OSV            string          `json:"osv,omitempty"`
	HWV            string          `json:"hwv,omitempty"`
	H              int             `json:"h,omitempty"`
	W              int             `json:"w,omitempty"`
	PPI            int             `json:"ppi,omitempty"`
	PxRatio        float64         `json:"pxratio,omitempty"`
	JS             int             `json:"js,omitempty"`
	GeoFetch       int             `json:"geofetch,omitempty"`
	FlashVer       string          `json:"flashver,omitempty"`
	Language       string          `json:"language,omitempty"`
	LangB          string          `json:"langb,omitempty"` // 2.6: BCP-47 language
	Carrier        string          `json:"carrier,omitempty"`
	MCCMNC         string          `json:"mccmnc,omitempty"`
	ConnectionType int             `json:"connectiontype,omitempty"`
	IFA            string          `json:"ifa,omitempty"`
	IDSHA1         string          `json:"didsha1,omitempty"`
	IDMD5          string          `json:"didmd5,omitempty"`
	DPIDSHA1       string          `json:"dpidsha1,omitempty"`
	DPIDMD5        string          `json:"dpidmd5,omitempty"`
	MacSHA1        string          `json:"macsha1,omitempty"`
	MacMD5         string          `json:"macmd5,omitempty"`
	Ext            json.RawMessage `json:"ext,omitempty"`
}

// UserAgent represents a structured user agent (2.6)
// Replaces the legacy ua string as browsers freeze UA strings
type UserAgent struct {
	Browsers []BrandVersion  `json:"browsers,omitempty"` // Browser brands and versions
	Platform *BrandVersion   `json:"platform,omitempty"` // Platform/OS brand and version
	Mobile   *int            `json:"mobile,omitempty"`   // 1 if mobile device
	Architecture string      `json:"architecture,omitempty"` // Device architecture (e.g. x86, arm)
	Bitness  string          `json:"bitness,omitempty"`  // Device bitness (e.g. 64)
	Model    string          `json:"model,omitempty"`    // Device model
	Source   int             `json:"source,omitempty"`   // Source of data: 0=unknown, 1=UA client hints (low entropy), 2=UA client hints (high entropy), 3=parsed from UA string
	Ext      json.RawMessage `json:"ext,omitempty"`
}

// BrandVersion represents a brand and version pair (2.6)
// Used in UserAgent for browser and platform identification
type BrandVersion struct {
	Brand   string   `json:"brand,omitempty"`   // Brand name (e.g. "Chrome", "Windows")
	Version []string `json:"version,omitempty"` // Version components (e.g. ["102", "0", "5005", "63"])
	Ext     json.RawMessage `json:"ext,omitempty"`
}

// Geo represents geographic location
type Geo struct {
	Lat           float64         `json:"lat,omitempty"`
	Lon           float64         `json:"lon,omitempty"`
	Type          int             `json:"type,omitempty"`
	Accuracy      int             `json:"accuracy,omitempty"`
	LastFix       int             `json:"lastfix,omitempty"`
	IPService     int             `json:"ipservice,omitempty"`
	Country       string          `json:"country,omitempty"`
	Region        string          `json:"region,omitempty"`
	RegionFIPS104 string          `json:"regionfips104,omitempty"`
	Metro         string          `json:"metro,omitempty"`
	City          string          `json:"city,omitempty"`
	ZIP           string          `json:"zip,omitempty"`
	UTCOffset     int             `json:"utcoffset,omitempty"`
	Ext           json.RawMessage `json:"ext,omitempty"`
}

// User represents a user
type User struct {
	ID         string          `json:"id,omitempty"`
	BuyerUID   string          `json:"buyeruid,omitempty"`
	YOB        int             `json:"yob,omitempty"`
	Gender     string          `json:"gender,omitempty"`
	Keywords   string          `json:"keywords,omitempty"`
	KwArray    []string        `json:"kwarray,omitempty"` // 2.6: Array of keywords
	CustomData string          `json:"customdata,omitempty"`
	Geo        *Geo            `json:"geo,omitempty"`
	Data       []Data          `json:"data,omitempty"`
	Consent    string          `json:"consent,omitempty"`
	EIDs       []EID           `json:"eids,omitempty"`
	Ext        json.RawMessage `json:"ext,omitempty"`
}

// Data represents data segment
type Data struct {
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	Segment []Segment       `json:"segment,omitempty"`
	Ext     json.RawMessage `json:"ext,omitempty"`
}

// Segment represents a data segment
type Segment struct {
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Value string          `json:"value,omitempty"`
	Ext   json.RawMessage `json:"ext,omitempty"`
}

// EID represents extended identifier
type EID struct {
	Source string          `json:"source,omitempty"`
	UIDs   []UID           `json:"uids,omitempty"`
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// UID represents a user ID
type UID struct {
	ID    string          `json:"id,omitempty"`
	AType int             `json:"atype,omitempty"`
	Ext   json.RawMessage `json:"ext,omitempty"`
}

// Source represents request source
type Source struct {
	FD     int             `json:"fd,omitempty"`
	TID    string          `json:"tid,omitempty"`
	PChain string          `json:"pchain,omitempty"`
	SChain *SupplyChain    `json:"schain,omitempty"` // 2.6: First-class (was ext in 2.5)
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// SupplyChain represents supply chain
type SupplyChain struct {
	Complete int               `json:"complete,omitempty"`
	Nodes    []SupplyChainNode `json:"nodes,omitempty"`
	Ver      string            `json:"ver,omitempty"`
	Ext      json.RawMessage   `json:"ext,omitempty"`
}

// SupplyChainNode represents a node in supply chain
type SupplyChainNode struct {
	ASI    string          `json:"asi,omitempty"`
	SID    string          `json:"sid,omitempty"`
	RID    string          `json:"rid,omitempty"`
	Name   string          `json:"name,omitempty"`
	Domain string          `json:"domain,omitempty"`
	HP     int             `json:"hp,omitempty"` // 2.6: Optional (was required in 2.5)
	Ext    json.RawMessage `json:"ext,omitempty"`
}

// Regs represents regulations
type Regs struct {
	COPPA     int             `json:"coppa,omitempty"`
	GDPR      *int            `json:"gdpr,omitempty"`
	USPrivacy string          `json:"us_privacy,omitempty"`
	GPP       string          `json:"gpp,omitempty"`
	GPPSID    []int           `json:"gpp_sid,omitempty"`
	Ext       json.RawMessage `json:"ext,omitempty"`
}

// Video placement type constants (2.6)
const (
	VideoPlcmtInstream           = 1 // Pre/mid/post-roll in a video player
	VideoPlcmtAccompanyingContent = 2 // Alongside primary content (was "in-banner" in 2.5)
	VideoPlcmtInterstitial       = 3 // Full-screen overlay
	VideoPlcmtNoContent          = 4 // Standalone, no primary content (was "in-feed/in-article")
)

// SSAI status constants (2.6)
const (
	SSAIUndetermined = 0 // Status unknown
	SSAIStitchedServer = 1 // Ad creative is stitched server-side
	SSAIStitchedClient = 2 // Ad creative is stitched/rendered client-side
)

// Device type constants relevant to inventory classification
const (
	DeviceTypeMobile       = 1 // Mobile/Tablet
	DeviceTypePC           = 2 // Personal Computer
	DeviceTypeCTV          = 3 // Connected TV
	DeviceTypePhone        = 4 // Phone
	DeviceTypeTablet       = 5 // Tablet
	DeviceTypeConnected    = 6 // Connected Device
	DeviceTypeSetTopBox    = 7 // Set Top Box
	DeviceTypeOOH          = 8 // OOH Device (2.6, used with DOOH)
)

// Category taxonomy constants (2.6)
const (
	CatTaxIAB10    = 1 // IAB Content Category Taxonomy 1.0 (default)
	CatTaxIAB20    = 2 // IAB Content Category Taxonomy 2.0
	CatTaxIABProd  = 3 // IAB Ad Product Taxonomy 1.0
	CatTaxIABAud   = 4 // IAB Audience Taxonomy 1.1
	CatTaxIABCont3 = 5 // IAB Content Category Taxonomy 3.0
	CatTaxIABCont31 = 6 // IAB Content Category Taxonomy 3.1 (latest)
)

// Qty source type constants (2.6)
const (
	QtySourceUnknown    = 0
	QtySourceMeasured   = 1 // Measured via device or sensor
	QtySourceProjected  = 2 // Projected from historical/model data
)
