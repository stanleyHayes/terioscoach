import { ImageResponse } from "next/og";

export const alt = "Terios Wellness Spa — clinical calm, wherever you are";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OpenGraphImage() {
  return new ImageResponse(
    <div style={{ width: "100%", height: "100%", display: "flex", position: "relative", overflow: "hidden", background: "#17352a", color: "#fbf7ed", padding: "68px 76px", fontFamily: "Arial, sans-serif" }}>
      <div style={{ position: "absolute", width: 430, height: 430, right: -90, top: -150, border: "1px solid rgba(199,222,208,.28)", borderRadius: "44% 56% 62% 38%", transform: "rotate(18deg)" }} />
      <div style={{ position: "absolute", width: 280, height: 280, left: -100, bottom: -140, background: "rgba(210,151,116,.16)", borderRadius: "50%" }} />
      <div style={{ display: "flex", flexDirection: "column", justifyContent: "space-between", width: "100%" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 18, fontSize: 27, fontWeight: 700 }}>
          <div style={{ display: "flex", width: 48, height: 48, alignItems: "center", justifyContent: "center", borderRadius: 15, background: "#fbf7ed", color: "#17352a", fontSize: 25 }}>T</div>
          Terios <span style={{ color: "#a9c9b7", marginLeft: -10, fontWeight: 500 }}>Wellness</span>
        </div>
        <div style={{ display: "flex", flexDirection: "column", maxWidth: 860 }}>
          <div style={{ fontSize: 16, textTransform: "uppercase", letterSpacing: "0.18em", color: "#d69c79", fontWeight: 700 }}>One-to-one care · Online</div>
          <div style={{ display: "flex", flexDirection: "column", marginTop: 20, fontSize: 70, lineHeight: 1.02, letterSpacing: "-0.045em", fontWeight: 700 }}><span>Clinical calm,</span><span>wherever you are.</span></div>
          <div style={{ marginTop: 26, fontSize: 22, lineHeight: 1.45, color: "#c7ded0" }}>Nursing consultations, wellness coaching and recovery support—held with clarity and care.</div>
        </div>
      </div>
    </div>,
    size,
  );
}
