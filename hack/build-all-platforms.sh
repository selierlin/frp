#!/usr/bin/env bash
set -e

LDFLAGS="-s -w"
OUTPUT_DIR="bin/release"
NOWEB_TAG=""

# 检查 web 资源是否存在，不存在则自动构建
if [ ! -d "web/frps/dist" ] || [ ! -d "web/frpc/dist" ]; then
    echo "web 资源未构建，正在执行 make web..."
    make web
    # 再次检查，如果构建失败则使用 noweb 标签
    if [ ! -d "web/frps/dist" ] || [ ! -d "web/frpc/dist" ]; then
        NOWEB_TAG=",noweb"
        echo "警告: web 资源构建失败，使用 noweb 标签"
    else
        echo "web 资源构建成功"
    fi
fi

mkdir -p "$OUTPUT_DIR"

PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm/7"
    "windows/amd64"
    "windows/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=$(echo "$PLATFORM" | cut -d'/' -f1)
    GOARCH=$(echo "$PLATFORM" | cut -d'/' -f2)
    GOARM=$(echo "$PLATFORM" | cut -d'/' -f3)

    SUFFIX=""
    [ "$GOOS" = "windows" ] && SUFFIX=".exe"

    ARM_TAG=""
    ARM_LABEL=""
    if [ -n "$GOARM" ]; then
        ARM_TAG="GOARM=$GOARM"
        ARM_LABEL="_v${GOARM}"
    fi

    LABEL="${GOOS}_${GOARCH}${ARM_LABEL}"

    echo "编译 frps -> $LABEL ..."
    env CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH $ARM_TAG \
        go build -trimpath -ldflags "$LDFLAGS" \
        -tags "frps${NOWEB_TAG}" \
        -o "${OUTPUT_DIR}/frps_${LABEL}${SUFFIX}" \
        ./cmd/frps

    echo "编译 frpc -> $LABEL ..."
    env CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH $ARM_TAG \
        go build -trimpath -ldflags "$LDFLAGS" \
        -tags "frpc${NOWEB_TAG}" \
        -o "${OUTPUT_DIR}/frpc_${LABEL}${SUFFIX}" \
        ./cmd/frpc
done

echo ""
echo "全部编译完成，输出目录: $OUTPUT_DIR"
ls -lh "$OUTPUT_DIR"
