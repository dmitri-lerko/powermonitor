#import <Foundation/Foundation.h>
#import <Vision/Vision.h>
#import <CoreGraphics/CoreGraphics.h>

int main(int argc, const char * argv[]) {
    @autoreleasepool {
        if (argc < 2) {
            NSLog(@"Usage: ane_load <iterations>");
            return 1;
        }
        
        int iterations = (int)strtol(argv[1], NULL, 10);
        int completed = 0;
        
        for (int i = 0; i < iterations; i++) {
            // Create a simple test image (RGBA pixels)
            int width = 128;
            int height = 128;
            size_t bytesPerRow = width * 4;
            uint8_t *pixelData = (uint8_t *)malloc(bytesPerRow * height);
            if (!pixelData) continue;
            
            // Fill with pattern for better feature extraction
            for (int y = 0; y < height; y++) {
                for (int x = 0; x < width; x++) {
                    pixelData[(y * bytesPerRow + x * 4) + 0] = (uint8_t)((x + y + i) % 256);
                    pixelData[(y * bytesPerRow + x * 4) + 1] = (uint8_t)((x * 2 + y + i) % 256);
                    pixelData[(y * bytesPerRow + x * 4) + 2] = (uint8_t)((x + y * 2 + i) % 256);
                    pixelData[(y * bytesPerRow + x * 4) + 3] = 255;
                }
            }
            
            CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
            CGDataProviderRef provider = CGDataProviderCreateWithData(NULL, pixelData, bytesPerRow * height, NULL);
            CGImageRef image = CGImageCreate(width, height, 8, 32, bytesPerRow, colorSpace,
                                              kCGBitmapByteOrder32Big | kCGImageAlphaPremultipliedLast,
                                              provider, NULL, false, kCGRenderingIntentDefault);
            CGDataProviderRelease(provider);
            CGColorSpaceRelease(colorSpace);
            free(pixelData);
            
            if (!image) continue;
            
            // Create a feature print request (triggers ANE)
            NSError *error = nil;
            VNGenerateImageFeaturePrintRequest *request = [[VNGenerateImageFeaturePrintRequest alloc] init];
            request.imageCropAndScaleOption = VNImageCropAndScaleOptionCenterCrop;
            
            // Perform the request
            VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];
            [handler performRequests:@[request] error:&error];
            
            CGImageRelease(image);
            
            if (error) {
                break;
            }
            
            completed++;
            
            // Small delay to allow powermetrics to capture the load
            [NSThread sleepForTimeInterval:0.05];
        }
        
        // Output the number of completed iterations
        printf("%d\n", completed);
        fflush(stdout);
        
        return 0;
    }
}
