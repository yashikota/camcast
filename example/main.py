import cv2

rtsp_url = "rtsp://localhost:8554/stream"
cap = cv2.VideoCapture(rtsp_url)

if not cap.isOpened():
    print("RTSP ストリームに接続できませんでした:", rtsp_url)
    exit()

print("RTSP ストリームからフレームを受信中…")

while True:
    ret, frame = cap.read()
    if not ret:
        print("フレームの読み込みに失敗しました")
        break

    cv2.imshow("RTSP Stream", frame)

    if cv2.waitKey(1) & 0xFF == ord('q'):
        break

cap.release()
cv2.destroyAllWindows()
