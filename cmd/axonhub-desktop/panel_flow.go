// 职责: 流量页的流向图（调用方 → Hub → 渠道）和请求行下方的尝试时间线。
// 关联文件:
//   cmd/axonhub-desktop/panel_view.go
//   cmd/axonhub-desktop/panel_load.go
//   cmd/axonhub-desktop/panel_present.go
//   cmd/axonhub-desktop/locales/en.json
//   cmd/axonhub-desktop/locales/zh-Hans.json
// 遵循的 Rules:
//   .agents/rules/go-general.md
// 注意事项:
//   流向图只由最近请求的快照推导，没有事件推送；光点表示"该链路上有进行中的请求"，不是逐请求轨迹。
//   动效沿用 magpie 路由舞台的语言：静止实线、选中点亮、进行中虚线流动、请求是带光晕的实心点。
//   两侧的节点各占等高的一格，画线时按同样的等分算 y，两边必须保持一致。

package main

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
)

const (
	flowNodeLimit          = 4
	flowSlotHeight         = 32
	flowHub        float32 = 15
	flowLinkGap    float32 = 6
	flowDash       float32 = 3
	flowDashGap    float32 = 5
	flowDashTime           = 700 * time.Millisecond
	packetTime             = 3200 * time.Millisecond
	packetFly      float32 = 0.34
	packetBreaths          = 2
)

// flowNode is a caller or a channel on the flow. Active means a request
// through it is in progress. Selected means the chosen request uses it.
// Tone is the result of its newest request.
type flowNode struct {
	Label    string
	Active   bool
	Selected bool
	Tone     statusTone
}

// panelFlow is both sides of the hub, newest first, each cut to the
// nodes that fit.
type panelFlow struct {
	Callers  []flowNode
	Channels []flowNode
}

func (f panelFlow) active() bool {
	return lo.SomeBy(f.Callers, func(n flowNode) bool { return n.Active })
}

// buildFlow reads the callers and channels out of the recent requests,
// which arrive newest first. A node takes the tone of its newest request.
func buildFlow(requests []panelRequest, selectedID string) panelFlow {
	return panelFlow{
		Callers:  flowSide(requests, selectedID, func(r panelRequest) string { return r.Caller }),
		Channels: flowSide(requests, selectedID, func(r panelRequest) string { return r.Channel }),
	}
}

func flowSide(requests []panelRequest, selectedID string, label func(panelRequest) string) []flowNode {
	var nodes []flowNode
	index := map[string]int{}
	for _, request := range requests {
		name := strings.TrimSpace(label(request))
		if name == "" {
			continue
		}
		i, ok := index[name]
		if !ok {
			i = len(nodes)
			index[name] = i
			nodes = append(nodes, flowNode{Label: name, Tone: presentRequest(request).Tone})
		}
		nodes[i].Active = nodes[i].Active || request.InProgress
		nodes[i].Selected = nodes[i].Selected || (selectedID != "" && request.ID == selectedID)
	}
	return lo.Slice(nodes, 0, flowNodeLimit)
}

func (s *shell) flowCard(c *ui.Context, p panelPalette) {
	request, selected := findRequest(s.snapshot.Requests, s.selectedID)
	label, tone := routingStatus(request, selected)
	flow := buildFlow(s.snapshot.Requests, s.selectedID)
	rows := max(len(flow.Callers), len(flow.Channels), 1)
	card(c.Key("flow"), p).Padding(12).Gap(10).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			sectionLabel(c, p, locales.Routing())
			ui.Spacer(c)
			chip(c, p, label, tone)
		})
		ui.Row(c).Height(float32(max(rows, 2) * flowSlotHeight)).AlignItems(ui.Stretch).Children(func() {
			flowColumn(c.Key("callers"), p, flow.Callers, locales.YourClient(), false)
			flowCanvas(c.Key("canvas"), p, flow)
			flowColumn(c.Key("channels"), p, flow.Channels, locales.Channel(), true)
		})
		ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
			if !selected {
				ui.Text(c, locales.RoutingIdle()).FontSize(11.5).TextColor(p.muted).SingleLine()
				return
			}
			if request.ModelID != "" {
				ui.Text(c, request.ModelID).Font("monospace").FontSize(11.5).TextColor(p.soft).SingleLine().Shrink(1)
			}
			ui.Spacer(c)
			ui.Text(c, routingSummary(request)).FontSize(11.5).FontFeatures("tnum").TextColor(p.muted).SingleLine().Shrink(0)
		})
	})
}

// flowColumn stacks one side's nodes in equal slots. With no node yet it
// shows one outlined placeholder.
func flowColumn(c *ui.Context, p panelPalette, nodes []flowNode, placeholder string, channels bool) {
	align := ui.Start
	if channels {
		align = ui.End
	}
	ui.Column(c).Shrink(1).MinWidth(0).MaxWidth(132).Children(func() {
		if len(nodes) == 0 {
			ui.Row(c).Grow(1).Basis(0).AlignItems(ui.Center).Justify(align).Children(func() {
				endpointPill(c, p, placeholder, ui.Color{}, false, true)
			})
			return
		}
		for i, node := range nodes {
			ui.Row(c.Key(i)).Grow(1).Basis(0).AlignItems(ui.Center).Justify(align).Children(func() {
				dotColor := ui.Color{}
				if channels {
					dotColor, _ = p.tone(node.Tone)
				}
				endpointPill(c, p, node.Label, dotColor, node.Selected, false)
			})
		}
	})
}

// flowCanvas draws the links between the columns and the hub, in the
// vocabulary of magpie's routing stage: a link at rest is a plain line,
// the chosen request's way is lit in gold, and a link with a request in
// flight is a dashed line flowing toward the channel. The request itself
// is a glowing dot that flies from its caller through the hub and then
// waits at its channel, breathing, until the cycle starts again.
func flowCanvas(c *ui.Context, p panelPalette, flow panelFlow) {
	canvas := ui.Box(c).Grow(1).Basis(0).MinWidth(72)
	dash, cycle := float32(0), float32(0)
	if flow.active() {
		dash = canvas.Loop("dash", flowDashTime, ui.Linear)
		cycle = canvas.Loop("packet", packetTime, ui.Linear)
	}
	canvas.Draw(func(pt *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		side := func(nodes []flowNode, edgeX, hubX float32, toHub bool) {
			count := max(len(nodes), 1)
			for i := range count {
				y := r.Y + (float32(i)+0.5)*r.H/float32(count)
				var node flowNode
				if i < len(nodes) {
					node = nodes[i]
				}
				link := flowLink{from: [2]float32{hubX, cy}, to: [2]float32{edgeX, y}}
				if toHub {
					link.from, link.to = link.to, link.from
				}
				link.paint(pt, p, node, len(nodes) == 0, dash)
				if node.Active {
					at, alpha, ok := packetAt(cycle, toHub)
					if ok {
						link.packet(pt, p, at, alpha)
					}
				}
			}
		}
		side(flow.Callers, r.X+flowLinkGap, cx-flowHub-flowLinkGap, true)
		side(flow.Channels, r.X+r.W-flowLinkGap, cx+flowHub+flowLinkGap, false)
		// The hub sits on a quiet ring, as magpie's hub card does.
		var ring ui.Path
		ring.Circle(cx, cy, flowHub+3)
		pt.FillPath(&ring, p.raised)
		paintLogo(pt, ui.Rect{X: cx - flowHub + 2, Y: cy - flowHub + 2, W: 2*flowHub - 4, H: 2*flowHub - 4}, p.logo, p.gold)
	})
}

// flowLink is one curve between a node and the hub, in the direction a
// request travels it.
type flowLink struct {
	from, to [2]float32
}

func (l flowLink) controls() (c1, c2 [2]float32) {
	mid := (l.from[0] + l.to[0]) / 2
	return [2]float32{mid, l.from[1]}, [2]float32{mid, l.to[1]}
}

func (l flowLink) paint(pt *ui.Painter, p panelPalette, node flowNode, placeholder bool, dash float32) {
	c1, c2 := l.controls()
	var path ui.Path
	path.MoveTo(l.from[0], l.from[1]).CubeTo(c1[0], c1[1], c2[0], c2[1], l.to[0], l.to[1])
	ink := p.wire.Alpha(0.4)
	switch {
	case placeholder:
		path.Dash(0, 0.01, 7).Cap(ui.RoundCap)
	case node.Active:
		ink = p.gold.Alpha(0.9)
		path.Dash(-dash*(flowDash+flowDashGap), flowDash, flowDashGap).Cap(ui.ButtCap)
	case node.Selected:
		ink = p.gold.Alpha(0.7)
	}
	pt.StrokePath(&path, 1.5, ink)
}

// packet draws the request at t along the link: a filled dot in a soft
// glow.
func (l flowLink) packet(pt *ui.Painter, p panelPalette, t, alpha float32) {
	c1, c2 := l.controls()
	x, y := cubicPoint(l.from, c1, c2, l.to, t)
	var glow, dot ui.Path
	glow.Circle(x, y, 6)
	pt.FillPath(&glow, p.gold.Alpha(0.22*alpha))
	dot.Circle(x, y, 3)
	pt.FillPath(&dot, p.gold.Alpha(alpha))
}

// packetAt places the request for one link at a point of the cycle. The
// caller's link carries it first, then the channel's, each eased; for
// the rest of the cycle it waits at the end of the channel's link and
// its opacity breathes. ok is false while the packet is on the other
// link.
func packetAt(cycle float32, callerLink bool) (t, alpha float32, ok bool) {
	half := packetFly / 2
	switch {
	case cycle < half:
		return easeInOut(cycle / half), 1, callerLink
	case cycle < packetFly:
		return easeInOut((cycle - half) / half), 1, !callerLink
	case callerLink:
		return 0, 0, false
	default:
		wait := (cycle - packetFly) / (1 - packetFly)
		breath := 0.5 + 0.5*float32(math.Cos(2*math.Pi*float64(wait)*packetBreaths))
		return 1, 0.45 + 0.55*breath, true
	}
}

// easeInOut is a smoothstep: the packet leaves and arrives slowly.
func easeInOut(t float32) float32 {
	return t * t * (3 - 2*t)
}

// cubicPoint is the point at t on a cubic Bézier.
func cubicPoint(p0, p1, p2, p3 [2]float32, t float32) (float32, float32) {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return a*p0[0] + b*p1[0] + c*p2[0] + d*p3[0], a*p0[1] + b*p1[1] + c*p2[1] + d*p3[1]
}

// attemptLine is one attempt as the detail draws it.
type attemptLine struct {
	Channel string
	Result  string
	Metrics string
	Error   string
	Tone    statusTone
}

// presentAttempt puts the upstream status code first when there is one,
// then the latency and the time to the first token.
func presentAttempt(attempt panelAttempt) attemptLine {
	line := attemptLine{Channel: panelLabel(attempt.Channel), Error: attempt.Error, Tone: toneMuted}
	switch attempt.Status {
	case statusCompleted:
		line.Tone = toneSuccess
	case statusFailed, statusCanceled:
		line.Tone = toneDanger
	case statusPending, statusProcessing:
		line.Tone = toneActive
	}
	line.Result = statusText(attempt.Status)
	if attempt.StatusCode != nil {
		line.Result = strconv.Itoa(*attempt.StatusCode)
	}
	var metrics []string
	if attempt.LatencyMs != nil {
		metrics = append(metrics, formatLatency(*attempt.LatencyMs))
	}
	if attempt.FirstTokenMs != nil {
		metrics = append(metrics, locales.FirstToken(formatLatency(*attempt.FirstTokenMs)))
	}
	line.Metrics = strings.Join(metrics, " · ")
	return line
}

// attemptErrors joins the attempts' errors for Copy error, one per line,
// each led by its channel.
func attemptErrors(attempts []panelAttempt) string {
	lines := lo.FilterMap(attempts, func(attempt panelAttempt, _ int) (string, bool) {
		if attempt.Error == "" {
			return "", false
		}
		return panelLabel(attempt.Channel) + ": " + attempt.Error, true
	})
	return strings.Join(lines, "\n")
}

// requestDetail is the attempts of the opened request, oldest first, so
// a failover reads top to bottom.
func (s *shell) requestDetail(c *ui.Context, p panelPalette, request panelRequest) {
	ui.Column(c).Shrink(0).Margin(0, 0, 4, 0).Padding(10, 12).Gap(8).Radius(10).Background(p.card).
		InsetShadow(0, 0, 0, 1, p.edge).Transition(rowEnter).Children(func() {
		if len(request.Attempts) == 0 {
			ui.Text(c, locales.NoAttempts()).FontSize(11.5).TextColor(p.muted)
		}
		for i, attempt := range request.Attempts {
			line := presentAttempt(attempt)
			fg, bg := p.tone(line.Tone)
			ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Start).Children(func() {
				ui.Row(c).Size(18, 18).Shrink(0).Center().Radius(999).Background(bg).Children(func() {
					ui.Textf(c, "%d", i+1).FontSize(10.5).FontWeight(600).FontFeatures("tnum").TextColor(fg)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Baseline).Children(func() {
						ui.Text(c, line.Channel).FontSize(12).FontWeight(500).SingleLine().Shrink(1)
						ui.Text(c, line.Result).FontSize(11.5).FontFeatures("tnum").TextColor(fg).SingleLine().Shrink(0)
						ui.Spacer(c)
						if line.Metrics != "" {
							ui.Text(c, line.Metrics).FontSize(11).FontFeatures("tnum").TextColor(p.muted).SingleLine().Shrink(0)
						}
					})
					if line.Error != "" {
						ui.Text(c, line.Error).FontSize(11).TextColor(p.soft).MaxLines(3)
					}
				})
			})
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if errors := attemptErrors(request.Attempts); errors != "" {
				what := "error:" + request.ID
				label := locales.CopyError()
				if s.copied == what {
					label = locales.Copied()
				}
				if textButton(c.Key("copy-error"), p, label, p.raised, p.text).Height(26).PaddingX(10).Radius(7).Clicked() {
					s.copyText(errors, what)
				}
			}
			ui.Spacer(c)
			if textButton(c.Key("open"), p, locales.OpenInAdmin(), p.raised, p.text).Height(26).PaddingX(10).Radius(7).Clicked() {
				s.openRequest(request)
			}
		})
	})
}
