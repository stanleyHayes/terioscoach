/** One stable video track for both participants; source tracks remain owned by the call. */
export function createRecordingComposite(
  sources: () => [MediaStream | null, MediaStream | null],
  labels: [string, string] = ["Practitioner", "Client"],
) {
  const canvas = document.createElement("canvas");
  canvas.width = 1280;
  canvas.height = 720;
  const paint = canvas.getContext("2d");
  if (!paint || typeof canvas.captureStream !== "function") throw new Error("Canvas recording unavailable");
  const videos = labels.map(() => {
    const video = document.createElement("video");
    video.muted = true;
    video.autoplay = true;
    video.playsInline = true;
    return video;
  });
  const draw = () => {
    paint.fillStyle = "#182b22";
    paint.fillRect(0, 0, 1280, 720);
    sources().forEach((source, index) => {
      const video = videos[index];
      const track = source?.getVideoTracks()[0];
      // Device changes can replace tracks within the same MediaStream.
      const current = video.srcObject as MediaStream | null;
      if (current?.getVideoTracks()[0] !== track) {
        video.srcObject = track ? new MediaStream([track]) : null;
        if (track) void video.play().catch(() => {});
      }
      const x = index * 640 + 16;
      paint.fillStyle = "#0d1913";
      paint.fillRect(x, 16, 608, 632);
      if (track?.enabled && !track.muted && track.readyState === "live" && video.readyState >= 2 && video.videoWidth) {
        const scale = Math.min(608 / video.videoWidth, 632 / video.videoHeight);
        const width = video.videoWidth * scale, height = video.videoHeight * scale;
        paint.drawImage(video, x + (608 - width) / 2, 16 + (632 - height) / 2, width, height);
      } else {
        paint.fillStyle = "#e8eee9";
        paint.font = "24px sans-serif";
        paint.textAlign = "center";
        paint.fillText("Camera unavailable", x + 304, 332);
      }
      paint.fillStyle = "#e8eee9";
      paint.font = "24px sans-serif";
      paint.textAlign = "center";
      paint.fillText(labels[index], x + 304, 690);
    });
  };
  draw();
  const stream = canvas.captureStream(25);
  const timer = setInterval(draw, 40);
  return {
    stream,
    stop() {
      clearInterval(timer);
      stream.getTracks().forEach(track => track.stop());
      videos.forEach(video => { video.pause(); video.srcObject = null; });
    },
  };
}
