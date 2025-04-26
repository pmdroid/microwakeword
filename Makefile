# Configurable variables
TENSORFLOW_REPO = https://github.com/tensorflow/tensorflow.git
TENSORFLOW_VERSION = v2.19.0
KISSFFT_VERSION = febd4caeed32e33ad8b2e0bb5ea77542c40f18ec
KISSFFT_REPO = https://github.com/mborgerding/kissfft.git
INSTALL_PREFIX = /usr/local
BUILD_DIR = build
EXTRAS_DIR = extras

# Tools
WGET = wget
GIT = git
MKDIR = mkdir -p
CP = cp -r
RM = rm -rf
BAZEL = bazel

# Output files to check if build is already done
TF_LITE_LIB = $(BUILD_DIR)/tensorflow/bazel-bin/tensorflow/lite/c/libtensorflowlite_c.so
MICROFRONTEND_LIB = $(BUILD_DIR)/tensorflow/bazel-bin/tensorflow/lite/experimental/microfrontend/lib/libmicrofrontend.so

.PHONY: all clean install install_shared download build check_build

all: download build

# Download dependencies
download: download_tensorflow download_kissfft

download_tensorflow:
	@echo "Checking for TensorFlow..."
	@if [ ! -d "$(BUILD_DIR)/tensorflow" ]; then \
		echo "Cloning TensorFlow $(TENSORFLOW_VERSION)..."; \
		$(MKDIR) $(BUILD_DIR); \
		cd $(BUILD_DIR) && \
		$(GIT) clone --depth 1 -b $(TENSORFLOW_VERSION) $(TENSORFLOW_REPO) tensorflow; \
	else \
		echo "TensorFlow already downloaded, skipping..."; \
	fi

download_kissfft:
	@echo "Checking for KissFFT..."
	@if [ ! -d "$(BUILD_DIR)/kissfft" ]; then \
		echo "Downloading KissFFT..."; \
		cd $(BUILD_DIR) && \
		$(GIT) clone $(KISSFFT_REPO) kissfft && \
		cd kissfft && \
		$(GIT) checkout $(KISSFFT_VERSION); \
	else \
		echo "KissFFT already downloaded, skipping..."; \
	fi

# Check if build is already done
check_build:
	@echo "Checking if build is already done..."
	@if [ -f "$(TF_LITE_LIB)" ] && [ -f "$(MICROFRONTEND_LIB)" ]; then \
		echo "Build already completed, skipping build step."; \
		touch $(TF_LITE_LIB) $(MICROFRONTEND_LIB); \
		exit 0; \
	fi

# Build the library using the existing BUILD file
build: download check_build
	@if [ ! -f "$(TF_LITE_LIB)" ] || [ ! -f "$(MICROFRONTEND_LIB)" ]; then \
		echo "Building library..."; \
		# Copy the BUILD file to TensorFlow's microfrontend library directory \
		$(MKDIR) $(BUILD_DIR)/tensorflow/tensorflow/lite/experimental/microfrontend/lib; \
		$(CP) $(EXTRAS_DIR)/BUILD $(BUILD_DIR)/tensorflow/tensorflow/lite/experimental/microfrontend/lib/; \
		# Build the library using TensorFlow's Bazel \
		cd $(BUILD_DIR)/tensorflow && \
		$(BAZEL) build //tensorflow/lite/experimental/microfrontend/lib:microfrontend && \
		$(BAZEL) build //tensorflow/lite/c:tensorflowlite_c; \
	fi

install: build
	@echo "Installing shared library..."
	$(MKDIR) $(INSTALL_PREFIX)/lib
	$(CP) $(BUILD_DIR)/tensorflow/bazel-bin/tensorflow/lite/c/libtensorflowlite_c.so $(INSTALL_PREFIX)/lib/
	$(CP) $(BUILD_DIR)/tensorflow/bazel-bin/tensorflow/lite/experimental/microfrontend/lib/libmicrofrontend.so $(INSTALL_PREFIX)/lib/
	@echo "Shared library installation complete to $(INSTALL_PREFIX)/lib"

install_shared: build
	@echo "Installing shared library..."
	$(MKDIR) $(INSTALL_PREFIX)/lib
	$(CP) $(BUILD_DIR)/tensorflow/bazel-bin/tensorflow/lite/c/libtensorflowlite_c.so $(INSTALL_PREFIX)/lib/
	$(CP) $(BUILD_DIR)/tensorflow/bazel-bin/tensorflow/lite/experimental/microfrontend/lib/libmicrofrontend.so $(INSTALL_PREFIX)/lib/
	@echo "Shared library installation complete to $(INSTALL_PREFIX)/lib"

# Clean up
clean:
	$(RM) $(BUILD_DIR)
	@echo "Clean complete"

# Show help
help:
	@echo "Makefile for building audio feature vectors library"
	@echo ""
	@echo "Targets:"
	@echo "  all        - Download dependencies, build and install the library (default)"
	@echo "  download   - Download TensorFlow and KissFFT"
	@echo "  build      - Build the library (skips if already built)"
	@echo "  install    - Install the library to $(INSTALL_PREFIX)"
	@echo "  clean      - Remove build directory"
	@echo ""
	@echo "Configuration:"
	@echo "  TENSORFLOW_REPO    = $(TENSORFLOW_REPO)"
	@echo "  KISSFFT_VERSION    = $(KISSFFT_VERSION)"
	@echo "  INSTALL_PREFIX     = $(INSTALL_PREFIX)"
	@echo ""
	@echo "Example usage:"
	@echo "  make                   # Build and install with default settings"
	@echo "  make INSTALL_PREFIX=/opt  # Install to /opt instead of /usr/local"
	@echo "  sudo make install    # Install the shared (.so) library"