package camera

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AVFoundation -framework CoreMedia -framework CoreVideo -framework Foundation

#import <AVFoundation/AVFoundation.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <Foundation/Foundation.h>

// CaptureResult 单帧抓取结果
typedef struct {
    unsigned char* data;  // JPEG 数据指针
    int            size;  // 数据字节数
    int            error; // 0=成功，非0=失败
} CaptureResult;

// PhotoCaptureDelegate AVCapturePhotoOutput 的代理，用于同步接收拍照结果
@interface PhotoCaptureDelegate : NSObject <AVCapturePhotoCaptureDelegate>
@property (nonatomic, strong) NSData* jpegData;
@property (nonatomic, strong) dispatch_semaphore_t semaphore;
@end

@implementation PhotoCaptureDelegate
- (void)captureOutput:(AVCapturePhotoOutput*)output
    didFinishProcessingPhoto:(AVCapturePhoto*)photo
                       error:(NSError*)error {
    if (!error) {
        self.jpegData = [photo fileDataRepresentation];
    }
    dispatch_semaphore_signal(self.semaphore);
}
@end

// captureOneFrame 使用 AVFoundation 抓取摄像头单帧，返回 JPEG 数据
// 调用方负责 free(result.data)
static CaptureResult captureOneFrame(void) {
    CaptureResult result = {NULL, 0, 0};

    // 检查摄像头权限
    AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeVideo];
    if (status == AVAuthorizationStatusDenied || status == AVAuthorizationStatusRestricted) {
        result.error = 1;
        return result;
    }

    // 查找默认摄像头
    AVCaptureDevice* device = [AVCaptureDevice defaultDeviceWithMediaType:AVMediaTypeVideo];
    if (!device) {
        result.error = 2;
        return result;
    }

    // 创建 session
    AVCaptureSession* session = [[AVCaptureSession alloc] init];
    session.sessionPreset = AVCaptureSessionPreset640x480;

    // 添加输入
    NSError* inputError = nil;
    AVCaptureDeviceInput* input = [AVCaptureDeviceInput deviceInputWithDevice:device error:&inputError];
    if (!input || inputError) {
        result.error = 3;
        return result;
    }
    [session addInput:input];

    // 使用现代 AVCapturePhotoOutput（替代已废弃的 AVCaptureStillImageOutput）
    AVCapturePhotoOutput* photoOutput = [[AVCapturePhotoOutput alloc] init];
    if (![session canAddOutput:photoOutput]) {
        result.error = 4;
        return result;
    }
    [session addOutput:photoOutput];

    // 启动 session
    [session startRunning];

    // 轮询等待 session 真正进入 running 状态（最多等待 3 秒）
    // startRunning 是异步的，必须确认 isRunning 为 YES 后才能拍照，
    // 否则 capturePhotoWithSettings:delegate: 会因无活跃视频连接而抛出 NSException
    int waitCount = 0;
    while (!session.isRunning && waitCount < 30) {
        [NSThread sleepForTimeInterval:0.1];
        waitCount++;
    }
    if (!session.isRunning) {
        [session stopRunning];
        result.error = 7;
        return result;
    }

    // 额外等待摄像头预热，避免首帧全黑
    [NSThread sleepForTimeInterval:0.3];

    // 配置拍照参数（JPEG 格式）
    AVCapturePhotoSettings* settings = [AVCapturePhotoSettings
        photoSettingsWithFormat:@{AVVideoCodecKey: AVVideoCodecTypeJPEG}];

    // 创建代理并拍照
    // 用 @try/@catch 捕获 ObjC 异常（如 session 连接未就绪），
    // 避免未捕获的 NSException 穿透 CGO 边界导致进程 crash
    PhotoCaptureDelegate* delegate = [[PhotoCaptureDelegate alloc] init];
    delegate.semaphore = dispatch_semaphore_create(0);

    @try {
        [photoOutput capturePhotoWithSettings:settings delegate:delegate];
    } @catch (NSException* exception) {
        [session stopRunning];
        result.error = 8;
        return result;
    }

    // 最多等待 3 秒
    dispatch_semaphore_wait(delegate.semaphore, dispatch_time(DISPATCH_TIME_NOW, 3 * NSEC_PER_SEC));

    [session stopRunning];

    NSData* jpegData = delegate.jpegData;
    if (!jpegData || jpegData.length == 0) {
        result.error = 5;
        return result;
    }

    // 拷贝数据到 C 堆（Go 侧负责 free）
    result.size = (int)jpegData.length;
    result.data = (unsigned char*)malloc(result.size);
    if (!result.data) {
        result.error = 6;
        return result;
    }
    memcpy(result.data, jpegData.bytes, result.size);
    return result;
}

// requestCameraPermissionSync 请求摄像头权限并同步等待用户响应
// 返回 1 表示用户授权，0 表示拒绝或超时
static int requestCameraPermissionSync(void) {
    dispatch_semaphore_t sema = dispatch_semaphore_create(0);
    __block BOOL userGranted = NO;
    [AVCaptureDevice requestAccessForMediaType:AVMediaTypeVideo
                            completionHandler:^(BOOL granted) {
        userGranted = granted;
        dispatch_semaphore_signal(sema);
    }];
    // 最多等待 60 秒（给用户足够时间响应弹窗）
    dispatch_semaphore_wait(sema, dispatch_time(DISPATCH_TIME_NOW, 60 * NSEC_PER_SEC));
    return userGranted ? 1 : 0;
}

// hasCameraPermission 检查是否已有摄像头权限
static int hasCameraPermission(void) {
    AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeVideo];
    return (status == AVAuthorizationStatusAuthorized) ? 1 : 0;
}
*/
import "C"
import (
	"encoding/base64"
	"fmt"
	"unsafe"
)

// CaptureFrame 抓取摄像头单帧，返回 JPEG 的 base64 编码字符串
// 每次调用都会打开/关闭摄像头 session，适合低频调用（如每隔数秒一次）
func CaptureFrame() (string, error) {
	result := C.captureOneFrame()
	if result.error != 0 {
		return "", fmt.Errorf("摄像头抓帧失败，错误码: %d", int(result.error))
	}
	defer C.free(unsafe.Pointer(result.data))

	jpegBytes := C.GoBytes(unsafe.Pointer(result.data), result.size)
	encoded := base64.StdEncoding.EncodeToString(jpegBytes)
	return encoded, nil
}

// RequestPermission 触发系统摄像头权限弹窗并同步等待用户响应
// 返回 true 表示用户授权，false 表示拒绝或超时（最多等待 60 秒）
func RequestPermission() bool {
	granted := C.requestCameraPermissionSync()
	return granted == 1
}

// HasPermission 检查是否已获得摄像头权限
func HasPermission() bool {
	return C.hasCameraPermission() == 1
}
