import cv2

rtsp_url = "rtsp://localhost:8554/stream"
cap = cv2.VideoCapture(rtsp_url)

if not cap.isOpened():
    print("Cannot connect to RTSP stream:", rtsp_url)
    exit()

print("Receiving frames from RTSP stream...")

while True:
    ret, frame = cap.read()
    if not ret:
        print("Failed to read frame")
        break

    cv2.imshow("RTSP Stream", frame)

    if cv2.waitKey(1) & 0xFF == ord("q"):
        break

cap.release()
cv2.destroyAllWindows()
