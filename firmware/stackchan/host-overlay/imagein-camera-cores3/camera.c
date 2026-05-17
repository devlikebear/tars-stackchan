/*
 * CoreS3 camera host overlay for TARS Stack-chan.
 *
 * This keeps Moddable's ECMA-419 Camera JavaScript API but replaces the
 * legacy camera driver path with the official CoreS3/ESP-BSP style path:
 * existing shared I2C handle -> esp_video DVP init -> V4L2 frame dequeue.
 */

#include "xsmc.h"
#include "mc.xs.h"
#include "mc.defines.h"
#include "xsHost.h"

#include "builtinCommon.h"

#include "driver/i2c_master.h"
#include "esp_err.h"
#include "esp_log.h"
#include "esp_video_device.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "img_converters.h"
#include "soc/gpio_num.h"

#include <errno.h>
#include <fcntl.h>
#include <inttypes.h>
#include <linux/videodev2.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <unistd.h>

#if !CONFIG_ESP_VIDEO_ENABLE_DVP_VIDEO_DEVICE
	#error "CoreS3 camera overlay requires CONFIG_ESP_VIDEO_ENABLE_DVP_VIDEO_DEVICE"
#endif
#if CONFIG_ESP_VIDEO_ENABLE_MIPI_CSI_VIDEO_DEVICE || CONFIG_ESP_VIDEO_ENABLE_SPI_VIDEO_DEVICE || CONFIG_ESP_VIDEO_ENABLE_USB_UVC_VIDEO_DEVICE || CONFIG_ESP_VIDEO_ENABLE_HW_JPEG_VIDEO_DEVICE || CONFIG_ESP_VIDEO_ENABLE_HW_H264_VIDEO_DEVICE || CONFIG_ESP_VIDEO_ENABLE_ISP_VIDEO_DEVICE
	#error "CoreS3 camera overlay assumes esp_video DVP-only layout"
#endif

#ifndef MAP_FAILED
	#define MAP_FAILED ((void *)-1)
#endif

#ifndef MODDEF_CAMERA_POWERDOWN
	#define MODDEF_CAMERA_POWERDOWN -1
#endif
#ifndef MODDEF_CAMERA_RESET
	#define MODDEF_CAMERA_RESET -1
#endif
#ifndef MODDEF_CAMERA_XCLK
	#define MODDEF_CAMERA_XCLK -1
#endif
#ifndef MODDEF_CAMERA_PCLK
	#define MODDEF_CAMERA_PCLK 45
#endif
#ifndef MODDEF_CAMERA_HREF
	#define MODDEF_CAMERA_HREF 38
#endif
#ifndef MODDEF_CAMERA_VSYNC
	#define MODDEF_CAMERA_VSYNC 46
#endif
#ifndef MODDEF_CAMERA_SDA
	#define MODDEF_CAMERA_SDA 12
#endif
#ifndef MODDEF_CAMERA_SCL
	#define MODDEF_CAMERA_SCL 11
#endif
#ifndef MODDEF_CAMERA_I2C_PORT
	#define MODDEF_CAMERA_I2C_PORT 1
#endif
#ifndef MODDEF_CAMERA_D0
	#define MODDEF_CAMERA_D0 39
#endif
#ifndef MODDEF_CAMERA_D1
	#define MODDEF_CAMERA_D1 40
#endif
#ifndef MODDEF_CAMERA_D2
	#define MODDEF_CAMERA_D2 41
#endif
#ifndef MODDEF_CAMERA_D3
	#define MODDEF_CAMERA_D3 42
#endif
#ifndef MODDEF_CAMERA_D4
	#define MODDEF_CAMERA_D4 15
#endif
#ifndef MODDEF_CAMERA_D5
	#define MODDEF_CAMERA_D5 16
#endif
#ifndef MODDEF_CAMERA_D6
	#define MODDEF_CAMERA_D6 48
#endif
#ifndef MODDEF_CAMERA_D7
	#define MODDEF_CAMERA_D7 47
#endif
#ifndef MODDEF_CAMERA_XCLK_FREQ_HZ
	#define MODDEF_CAMERA_XCLK_FREQ_HZ 20000000
#endif
#ifndef MODDEF_CAMERA_JPEG_QUALITY
	#define MODDEF_CAMERA_JPEG_QUALITY 12
#endif

#define kCameraBufferCount 1
#define kCameraCaptureTries 3
#define kCameraDQBUFTimeoutMs 3000

// Moddable native modules are compiled by the SDK's host makefile, not CMake,
// so ESP-IDF transitive include directories from esp_video are not visible
// here. Keep a DVP-only mirror of the public esp_video init ABI; sdkconfig
// assertions above make the layout explicit and fail fast if it drifts.
#define ESP_VIDEO_INIT_FLAGS_DVP (1 << 1)
#define ESP_CAM_CTLR_DVP_DATA_SIG_NUM 16

typedef int cam_ctlr_data_width_t;

enum {
	CAM_CTLR_DATA_WIDTH_8 = 8
};

typedef struct esp_cam_ctlr_dvp_pin_config {
	cam_ctlr_data_width_t data_width;
	gpio_num_t data_io[ESP_CAM_CTLR_DVP_DATA_SIG_NUM];
	gpio_num_t vsync_io;
	gpio_num_t de_io;
	gpio_num_t pclk_io;
	gpio_num_t xclk_io;
} esp_cam_ctlr_dvp_pin_config_t;

typedef struct esp_video_init_sccb_config {
	bool init_sccb;
	union {
		struct {
			uint8_t port;
			gpio_num_t scl_pin;
			gpio_num_t sda_pin;
		} i2c_config;
		i2c_master_bus_handle_t i2c_handle;
	};
	uint32_t freq;
} esp_video_init_sccb_config_t;

typedef struct esp_video_init_dvp_config {
	esp_video_init_sccb_config_t sccb_config;
	gpio_num_t reset_pin;
	gpio_num_t pwdn_pin;
	esp_cam_ctlr_dvp_pin_config_t dvp_pin;
	uint32_t xclk_freq;
} esp_video_init_dvp_config_t;

typedef struct esp_video_init_config {
	const esp_video_init_dvp_config_t *dvp;
} esp_video_init_config_t;

esp_err_t esp_video_init_with_flags(const esp_video_init_config_t *config, uint32_t flags);
esp_err_t esp_video_deinit_with_flags(uint32_t flags);

static const char *TAG = "tars-camera";

typedef struct CameraVideoBufferRecord {
	void	*start;
	size_t	length;
} CameraVideoBufferRecord;

typedef struct CameraFrameRecord {
	uint8_t	*data;
	size_t	dataLength;
	xsSlot	*hostBuffer;
	uint8_t	state;
} CameraFrameRecord;

enum {
	kFrameFree = 0,
	kFrameReady,
	kFrameClient
};

typedef struct CameraRecord CameraRecord;
typedef struct CameraRecord *Camera;

struct CameraRecord {
	xsMachine	*the;
	xsSlot		object;
	xsSlot		*onReadable;
	xsSlot		*hostBufferPrototype;

	uint8_t		format;
	uint8_t		isJPEG;
	uint8_t		calling;
	uint8_t		closing;
	int			imageType;

	uint16_t	width;
	uint16_t	height;
	uint8_t		jpegQuality;

	i2c_master_bus_handle_t i2cBus;
	bool		ownsI2C;
	bool		videoInitialized;
	bool		streaming;
	int			videoFd;
	uint32_t	pixelFormat;

	uint8_t		videoBufferCount;
	CameraVideoBufferRecord buffers[kCameraBufferCount];
	CameraFrameRecord frame;

	const char	*lastErrorStep;
	esp_err_t	lastError;
	int			lastErrno;
};

static void xs_camera_mark(xsMachine *the, void *it, xsMarkRoot markRoot);
void xs_camera_destructor(void *data);

static const xsHostHooks ICACHE_RODATA_ATTR xsCameraHooks = {
	xs_camera_destructor,
	xs_camera_mark,
	NULL
};

static esp_err_t cameraGetI2C(Camera camera)
{
	esp_err_t err;

	err = i2c_master_get_bus_handle(I2C_NUM_0, &camera->i2cBus);
	if ((ESP_OK == err) && camera->i2cBus)
		return ESP_OK;

	err = i2c_master_get_bus_handle(I2C_NUM_1, &camera->i2cBus);
	if ((ESP_OK == err) && camera->i2cBus)
		return ESP_OK;

	i2c_master_bus_config_t busConfig = {
		.i2c_port = MODDEF_CAMERA_I2C_PORT,
		.sda_io_num = (gpio_num_t)MODDEF_CAMERA_SDA,
		.scl_io_num = (gpio_num_t)MODDEF_CAMERA_SCL,
		.clk_source = I2C_CLK_SRC_DEFAULT,
		.glitch_ignore_cnt = 7,
		.flags.enable_internal_pullup = true,
	};

	err = i2c_new_master_bus(&busConfig, &camera->i2cBus);
	if (ESP_OK == err)
		camera->ownsI2C = true;

	return err;
}

static esp_err_t cameraSetError(Camera camera, const char *step, esp_err_t err, int errNo)
{
	camera->lastErrorStep = step;
	camera->lastError = err;
	camera->lastErrno = errNo;
	return err;
}

static esp_err_t cameraSetErrno(Camera camera, const char *step)
{
	return cameraSetError(camera, step, ESP_FAIL, errno);
}

static int cameraFormatRank(uint32_t pixelFormat)
{
	switch (pixelFormat) {
		case V4L2_PIX_FMT_JPEG:
			return 0;
		case V4L2_PIX_FMT_RGB565:
			return 1;
#ifdef V4L2_PIX_FMT_RGB565X
		case V4L2_PIX_FMT_RGB565X:
			return 2;
#endif
		case V4L2_PIX_FMT_YUYV:
		case V4L2_PIX_FMT_YUV422P:
			return 3;
		case V4L2_PIX_FMT_RGB24:
			return 4;
		case V4L2_PIX_FMT_GREY:
			return 5;
		default:
			return 1000;
	}
}

static bool image_to_jpeg(uint8_t *src, size_t srcLen, uint16_t width, uint16_t height, uint32_t v4l2Format, uint8_t quality, uint8_t **out, size_t *outLen)
{
	pixformat_t format;

	*out = NULL;
	*outLen = 0;

	switch (v4l2Format) {
		case V4L2_PIX_FMT_JPEG:
			*out = malloc(srcLen);
			if (!*out)
				return false;
			memcpy(*out, src, srcLen);
			*outLen = srcLen;
			return true;

		case V4L2_PIX_FMT_RGB565:
#ifdef V4L2_PIX_FMT_RGB565X
		case V4L2_PIX_FMT_RGB565X:
#endif
			jpgSetRgb565BE(true);
			format = PIXFORMAT_RGB565;
			break;

		case V4L2_PIX_FMT_YUYV:
		case V4L2_PIX_FMT_YUV422P:
			// esp_video reports GC0308 YUV422P as packed YUYV on ESP32-S3.
			format = PIXFORMAT_YUV422;
			break;

		case V4L2_PIX_FMT_RGB24:
			format = PIXFORMAT_RGB888;
			break;

		case V4L2_PIX_FMT_GREY:
			format = PIXFORMAT_GRAYSCALE;
			break;

		default:
			ESP_LOGE(TAG, "unsupported V4L2 pixel format: 0x%08" PRIx32, v4l2Format);
			return false;
	}

	return fmt2jpg(src, srcLen, width, height, format, quality, out, outLen);
}

static void cameraFreeFrame(Camera camera, bool detachHostBuffer)
{
	CameraFrameRecord *frame = &camera->frame;

	if (detachHostBuffer && frame->hostBuffer) {
		xsMachine *the = camera->the;
		xsSlot tmp = xsReference(frame->hostBuffer);
		xsmcSetHostBuffer(tmp, NULL, 0);
	}

	if (frame->data)
		free(frame->data);

	frame->data = NULL;
	frame->dataLength = 0;
	frame->hostBuffer = NULL;
	frame->state = kFrameFree;
}

static esp_err_t cameraConfigureVideo(Camera camera)
{
	esp_err_t err;

	err = cameraGetI2C(camera);
	if (ESP_OK != err) {
		ESP_LOGE(TAG, "I2C bus unavailable: %s", esp_err_to_name(err));
		return cameraSetError(camera, "cameraGetI2C", err, 0);
	}

	esp_cam_ctlr_dvp_pin_config_t dvpPins = {
		.data_width = CAM_CTLR_DATA_WIDTH_8,
		.data_io = {
			(gpio_num_t)MODDEF_CAMERA_D0,
			(gpio_num_t)MODDEF_CAMERA_D1,
			(gpio_num_t)MODDEF_CAMERA_D2,
			(gpio_num_t)MODDEF_CAMERA_D3,
			(gpio_num_t)MODDEF_CAMERA_D4,
			(gpio_num_t)MODDEF_CAMERA_D5,
			(gpio_num_t)MODDEF_CAMERA_D6,
			(gpio_num_t)MODDEF_CAMERA_D7,
			GPIO_NUM_NC,
			GPIO_NUM_NC,
			GPIO_NUM_NC,
			GPIO_NUM_NC,
			GPIO_NUM_NC,
			GPIO_NUM_NC,
			GPIO_NUM_NC,
			GPIO_NUM_NC,
		},
		.vsync_io = (gpio_num_t)MODDEF_CAMERA_VSYNC,
		.de_io = (gpio_num_t)MODDEF_CAMERA_HREF,
		.pclk_io = (gpio_num_t)MODDEF_CAMERA_PCLK,
		.xclk_io = (gpio_num_t)MODDEF_CAMERA_XCLK,
	};

	esp_video_init_sccb_config_t sccbConfig = {
		.init_sccb = false,
		.i2c_handle = camera->i2cBus,
		.freq = 100000,
	};

	esp_video_init_dvp_config_t dvpConfig = {
		.sccb_config = sccbConfig,
		.reset_pin = (gpio_num_t)MODDEF_CAMERA_RESET,
		.pwdn_pin = (gpio_num_t)MODDEF_CAMERA_POWERDOWN,
		.dvp_pin = dvpPins,
		.xclk_freq = MODDEF_CAMERA_XCLK_FREQ_HZ,
	};

	esp_video_init_config_t videoConfig = {
		.dvp = &dvpConfig,
	};

	err = esp_video_init_with_flags(&videoConfig, ESP_VIDEO_INIT_FLAGS_DVP);
	if (ESP_OK != err) {
		ESP_LOGE(TAG, "esp_video_init failed: %s", esp_err_to_name(err));
		return cameraSetError(camera, "esp_video_init_with_flags", err, 0);
	}
	camera->videoInitialized = true;

	camera->videoFd = open(ESP_VIDEO_DVP_DEVICE_NAME, O_RDWR | O_NONBLOCK);
	if (camera->videoFd < 0) {
		ESP_LOGE(TAG, "open %s failed: errno=%d", ESP_VIDEO_DVP_DEVICE_NAME, errno);
		return cameraSetErrno(camera, "open DVP device");
	}

	struct v4l2_capability capability = {0};
	if (ioctl(camera->videoFd, VIDIOC_QUERYCAP, &capability) != 0) {
		ESP_LOGE(TAG, "VIDIOC_QUERYCAP failed: errno=%d", errno);
		return cameraSetErrno(camera, "VIDIOC_QUERYCAP");
	}

	struct v4l2_format format = {0};
	format.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	if (ioctl(camera->videoFd, VIDIOC_G_FMT, &format) != 0) {
		ESP_LOGE(TAG, "VIDIOC_G_FMT failed: errno=%d", errno);
		return cameraSetErrno(camera, "VIDIOC_G_FMT");
	}
	struct v4l2_format currentFormat = format;

	struct v4l2_fmtdesc desc = {0};
	uint32_t bestFormat = 0;
	int bestRank = 1000;
	desc.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	while (ioctl(camera->videoFd, VIDIOC_ENUM_FMT, &desc) == 0) {
		int rank = cameraFormatRank(desc.pixelformat);
		if (rank < bestRank) {
			bestRank = rank;
			bestFormat = desc.pixelformat;
		}
		desc.index++;
	}
	if (!bestFormat)
		bestFormat = format.fmt.pix.pixelformat;
	if (cameraFormatRank(bestFormat) >= 1000) {
		ESP_LOGE(TAG, "no supported V4L2 pixel format found");
		return cameraSetError(camera, "select V4L2 pixel format", ESP_FAIL, 0);
	}

	format.fmt.pix.width = camera->width;
	format.fmt.pix.height = camera->height;
	format.fmt.pix.pixelformat = bestFormat;
	if (ioctl(camera->videoFd, VIDIOC_S_FMT, &format) != 0) {
		int setErrno = errno;
		ESP_LOGE(TAG, "VIDIOC_S_FMT failed: errno=%d; falling back to current V4L2 format", setErrno);
		if (cameraFormatRank(currentFormat.fmt.pix.pixelformat) >= 1000)
			return cameraSetError(camera, "fallback V4L2 pixel format", ESP_FAIL, setErrno);
		format = currentFormat;
	}

	camera->width = (uint16_t)format.fmt.pix.width;
	camera->height = (uint16_t)format.fmt.pix.height;
	camera->pixelFormat = format.fmt.pix.pixelformat;

	struct v4l2_requestbuffers req = {0};
	req.count = kCameraBufferCount;
	req.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	req.memory = V4L2_MEMORY_MMAP;
	if (ioctl(camera->videoFd, VIDIOC_REQBUFS, &req) != 0) {
		ESP_LOGE(TAG, "VIDIOC_REQBUFS failed: errno=%d", errno);
		return cameraSetErrno(camera, "VIDIOC_REQBUFS");
	}
	if (req.count > kCameraBufferCount)
		req.count = kCameraBufferCount;
	camera->videoBufferCount = req.count;

	for (uint32_t i = 0; i < req.count; i++) {
		struct v4l2_buffer buffer = {0};
		buffer.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
		buffer.memory = V4L2_MEMORY_MMAP;
		buffer.index = i;
		if (ioctl(camera->videoFd, VIDIOC_QUERYBUF, &buffer) != 0) {
			ESP_LOGE(TAG, "VIDIOC_QUERYBUF failed: errno=%d", errno);
			return cameraSetErrno(camera, "VIDIOC_QUERYBUF");
		}

		void *start = mmap(NULL, buffer.length, PROT_READ | PROT_WRITE, MAP_SHARED, camera->videoFd, buffer.m.offset);
		if ((MAP_FAILED == start) || (NULL == start)) {
			ESP_LOGE(TAG, "mmap failed: errno=%d", errno);
			return cameraSetErrno(camera, "mmap V4L2 buffer");
		}

		camera->buffers[i].start = start;
		camera->buffers[i].length = buffer.length;

		if (ioctl(camera->videoFd, VIDIOC_QBUF, &buffer) != 0) {
			ESP_LOGE(TAG, "VIDIOC_QBUF failed: errno=%d", errno);
			return cameraSetErrno(camera, "VIDIOC_QBUF");
		}
	}

	int type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	if (ioctl(camera->videoFd, VIDIOC_STREAMON, &type) != 0) {
		ESP_LOGE(TAG, "VIDIOC_STREAMON failed: errno=%d", errno);
		return cameraSetErrno(camera, "VIDIOC_STREAMON");
	}
	camera->streaming = true;

	ESP_LOGI(TAG, "CoreS3 camera ready: %ux%u fourcc=0x%08" PRIx32, camera->width, camera->height, camera->pixelFormat);
	return ESP_OK;
}

static int cameraDequeueBuffer(Camera camera, struct v4l2_buffer *buffer)
{
	uint32_t waited = 0;

	while (waited <= kCameraDQBUFTimeoutMs) {
		if (ioctl(camera->videoFd, VIDIOC_DQBUF, buffer) == 0)
			return 0;

		if ((EAGAIN != errno) && (EWOULDBLOCK != errno)) {
			ESP_LOGE(TAG, "VIDIOC_DQBUF failed: errno=%d", errno);
			return -1;
		}

		vTaskDelay(pdMS_TO_TICKS(10));
		waited += 10;
	}

	ESP_LOGE(TAG, "VIDIOC_DQBUF timed out after %ums", (unsigned)kCameraDQBUFTimeoutMs);
	errno = ETIMEDOUT;
	return -1;
}

static void cameraTeardownVideo(Camera camera)
{
	if (camera->streaming && (camera->videoFd >= 0)) {
		int type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
		ioctl(camera->videoFd, VIDIOC_STREAMOFF, &type);
		camera->streaming = false;
	}

	for (uint8_t i = 0; i < camera->videoBufferCount; i++) {
		if (camera->buffers[i].start && camera->buffers[i].length) {
			munmap(camera->buffers[i].start, camera->buffers[i].length);
			camera->buffers[i].start = NULL;
			camera->buffers[i].length = 0;
		}
	}
	camera->videoBufferCount = 0;

	if (camera->videoFd >= 0) {
		close(camera->videoFd);
		camera->videoFd = -1;
	}

	if (camera->videoInitialized) {
		esp_video_deinit_with_flags(ESP_VIDEO_INIT_FLAGS_DVP);
		camera->videoInitialized = false;
	}

	if (camera->ownsI2C && camera->i2cBus) {
		i2c_del_master_bus(camera->i2cBus);
		camera->i2cBus = NULL;
		camera->ownsI2C = false;
	}
}

static esp_err_t cameraCaptureFrame(Camera camera)
{
	uint8_t *jpeg = NULL;
	size_t jpegLength = 0;
	esp_err_t result = ESP_FAIL;

	cameraFreeFrame(camera, true);

	for (int attempt = 0; attempt < kCameraCaptureTries; attempt++) {
		struct v4l2_buffer buffer = {0};
		buffer.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
		buffer.memory = V4L2_MEMORY_MMAP;
		if (cameraDequeueBuffer(camera, &buffer) != 0)
			return ESP_FAIL;

		bool useFrame = (attempt == (kCameraCaptureTries - 1));
		if (useFrame) {
			if (buffer.index >= camera->videoBufferCount) {
				ESP_LOGE(TAG, "invalid V4L2 buffer index: %lu", (unsigned long)buffer.index);
			}
			else {
				size_t bytesUsed = buffer.bytesused ? buffer.bytesused : camera->buffers[buffer.index].length;
				if (image_to_jpeg((uint8_t *)camera->buffers[buffer.index].start, bytesUsed, camera->width, camera->height, camera->pixelFormat, camera->jpegQuality, &jpeg, &jpegLength)) {
					camera->frame.data = jpeg;
					camera->frame.dataLength = jpegLength;
					camera->frame.state = kFrameReady;
					result = ESP_OK;
				}
				else
					ESP_LOGE(TAG, "image_to_jpeg failed");
			}
		}

		if (ioctl(camera->videoFd, VIDIOC_QBUF, &buffer) != 0) {
			ESP_LOGE(TAG, "VIDIOC_QBUF failed: errno=%d", errno);
			if (jpeg) {
				free(jpeg);
				camera->frame.data = NULL;
				camera->frame.dataLength = 0;
				camera->frame.state = kFrameFree;
			}
			return ESP_FAIL;
		}

		if (ESP_OK == result)
			return ESP_OK;
	}

	return result;
}

void xs_camera_constructor(xsMachine *the)
{
	uint32_t width = 320;
	uint32_t height = 240;
	uint8_t format = kIOFormatBuffer;
	Camera camera;
	int imageType = -1;
	uint8_t isJPEG = 1;

	xsmcVars(1);

	format = builtinInitializeFormat(the, format);
	if ((kIOFormatBuffer != format) && (kIOFormatBufferDisposable != format))
		xsRangeError("invalid format");

	if (xsmcHas(xsArg(0), xsID_width)) {
		xsmcGet(xsVar(0), xsArg(0), xsID_width);
		width = xsmcToInteger(xsVar(0));
	}
	if (xsmcHas(xsArg(0), xsID_height)) {
		xsmcGet(xsVar(0), xsArg(0), xsID_height);
		height = xsmcToInteger(xsVar(0));
	}
	if (xsmcHas(xsArg(0), xsID_imageType)) {
		xsmcGet(xsVar(0), xsArg(0), xsID_imageType);
		if ((xsStringType != xsmcTypeOf(xsVar(0))) || c_strcmp("jpeg", xsmcToString(xsVar(0))))
			xsRangeError("CoreS3 camera overlay supports imageType: 'jpeg'");
	}

	camera = c_calloc(1, sizeof(CameraRecord));
	if (!camera)
		xsUnknownError("not enough memory");
	camera->videoFd = -1;
	camera->the = the;
	camera->object = xsThis;
	camera->format = format;
	camera->imageType = imageType;
	camera->isJPEG = isJPEG;
	camera->width = (uint16_t)width;
	camera->height = (uint16_t)height;
	camera->jpegQuality = MODDEF_CAMERA_JPEG_QUALITY;

	xsmcSetHostData(xsThis, camera);
	xsSetHostHooks(xsThis, (xsHostHooks *)&xsCameraHooks);
	xsRemember(camera->object);

	camera->onReadable = builtinGetCallback(the, xsID_onReadable);
	builtinInitializeTarget(the);

	xsmcGet(xsVar(0), xsArg(0), xsID_prototype);
	camera->hostBufferPrototype = xsmcToReference(xsVar(0));

	esp_err_t err = cameraConfigureVideo(camera);
	if (ESP_OK != err) {
		char message[160];
		const char *step = camera->lastErrorStep ? camera->lastErrorStep : "cameraConfigureVideo";
		snprintf(message, sizeof(message), "camera init failed at %s: %s errno=%d", step, esp_err_to_name(err), camera->lastErrno);
		xsmcSetHostData(xsThis, NULL);
		xsmcSetHostDestructor(xsThis, NULL);
		xsForget(camera->object);
		xs_camera_destructor(camera);
		xsUnknownError(message);
	}
}

void xs_camera_destructor(void *it)
{
	if (!it)
		return;

	Camera camera = it;
	cameraFreeFrame(camera, false);
	cameraTeardownVideo(camera);
	c_free(camera);
}

void xs_camera_close(xsMachine *the)
{
	Camera camera = xsmcGetHostData(xsThis);
	if ((camera) && xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks)) {
		xsmcSetHostData(xsThis, NULL);
		xsmcSetHostDestructor(xsThis, NULL);
		xsForget(camera->object);
		if (camera->calling)
			camera->closing = 1;
		else
			xs_camera_destructor(camera);
	}
}

static void xs_camera_mark(xsMachine *the, void *it, xsMarkRoot markRoot)
{
	Camera camera = it;

	if (camera->onReadable)
		(*markRoot)(the, camera->onReadable);
	if (camera->hostBufferPrototype)
		(*markRoot)(the, camera->hostBufferPrototype);
	if (camera->frame.hostBuffer)
		(*markRoot)(the, camera->frame.hostBuffer);
}

void xs_camera_read(xsMachine *the)
{
	Camera camera = xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);
	CameraFrameRecord *frame = &camera->frame;

	if (kFrameReady != frame->state)
		return;

	if (kIOFormatBufferDisposable == camera->format) {
		xsSlot tmp;

		tmp = xsReference(camera->hostBufferPrototype);
		xsmcSetNewHostInstance(xsResult, tmp);
		xsmcSetHostBuffer(xsResult, frame->data, frame->dataLength);
		xsmcDefine(xsResult, xsID_camera, xsThis, xsDontDelete | xsDontSet);
		xsmcSetInteger(tmp, frame->dataLength);
		xsmcDefine(xsResult, xsID_byteLength, tmp, xsDontDelete | xsDontSet);
		xsmcPetrifyHostBuffer(xsResult);

		frame->hostBuffer = xsmcToReference(xsResult);
		frame->state = kFrameClient;
	}
	else {
		if ((xsmcArgc > 0) && (xsReferenceType == xsmcTypeOf(xsArg(0)))) {
			void *dst;
			xsUnsignedValue requested;

			xsmcGetBufferWritable(xsArg(0), &dst, &requested);
			if (requested < frame->dataLength)
				xsRangeError("buffer too small");
			memcpy(dst, frame->data, frame->dataLength);
			xsmcSetInteger(xsResult, frame->dataLength);
		}
		else
			xsmcSetArrayBuffer(xsResult, frame->data, frame->dataLength);

		cameraFreeFrame(camera, false);
	}
}

void xs_camera_start(xsMachine *the)
{
	Camera camera = xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);

	if (ESP_OK != cameraCaptureFrame(camera))
		xsUnknownError("camera capture failed");

	if (camera->onReadable) {
		camera->calling = 1;
		xsCallFunction0(xsReference(camera->onReadable), camera->object);
		camera->calling = 0;
		if (camera->closing)
			xs_camera_destructor(camera);
	}
}

void xs_camera_stop(xsMachine *the)
{
	Camera camera = xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);
	cameraFreeFrame(camera, true);
}

void xs_camera_get_format(xsMachine *the)
{
	Camera camera = xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);
	builtinGetFormat(the, camera->format);
}

void xs_camera_set_format(xsMachine *the)
{
	Camera camera = xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);
	uint8_t format = builtinSetFormat(the);
	if ((kIOFormatBuffer != format) && (kIOFormatBufferDisposable != format))
		xsRangeError("invalid format");
	camera->format = format;
}

void xs_camera_get_imageType(xsMachine *the)
{
	Camera camera = xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);
	if (camera->isJPEG)
		xsmcSetStringX(xsResult, "jpeg");
	else
		xsmcSetInteger(xsResult, camera->imageType);
}

void xs_camera_get_width(xsMachine *the)
{
	Camera camera = xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);
	xsmcSetInteger(xsResult, camera->width);
}

void xs_camera_get_height(xsMachine *the)
{
	Camera camera = xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);
	xsmcSetInteger(xsResult, camera->height);
}

void xs_camera_get_identification(xsMachine *the)
{
	(void)xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);

	xsmcVars(1);
	xsmcSetNewObject(xsResult);
	xsmcSetString(xsVar(0), "M5Stack CoreS3 GC0308 (esp_video)");
	xsmcSet(xsResult, xsID_model, xsVar(0));
}

void xs_camera_get_configuration(xsMachine *the)
{
	(void)xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);
	xsmcSetNewObject(xsResult);
}

void xs_camera_configure(xsMachine *the)
{
	(void)xsmcGetHostDataValidate(xsThis, (void *)&xsCameraHooks);
}

void _xs_disposable_hostbuffer_destructor(void *data)
{
}

void _xs_disposable_hostbuffer_close(xsMachine *the)
{
	void *buffer = xsmcGetHostData(xsThis);
	if (!buffer)
		return;

	xsmcVars(1);

	xsmcGet(xsVar(0), xsThis, xsID_camera);
	Camera camera = xsmcGetHostDataValidate(xsVar(0), (void *)&xsCameraHooks);
	CameraFrameRecord *frame = &camera->frame;

	if (frame->data != buffer)
		xsUnknownError("unknown buffer");

	xsmcSetHostBuffer(xsThis, NULL, 0);
	frame->hostBuffer = NULL;
	cameraFreeFrame(camera, false);
}
