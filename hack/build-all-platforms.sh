#!/usr/bin/env bash
set -e

LDFLAGS="-s -w"
OUTPUT_DIR="bin/release"
NOWEB_TAG=""

# 所有可选构建目标（格式: binary/goos/goarch[/goarm]）
ALL_TARGETS=(
    "frps/linux/amd64"
    "frps/linux/arm64"
    "frps/linux/arm/7"
    "frps/windows/amd64"
    "frps/windows/arm64"
    "frps/darwin/amd64"
    "frps/darwin/arm64"
    "frpc/linux/amd64"
    "frpc/linux/arm64"
    "frpc/linux/arm/7"
    "frpc/windows/amd64"
    "frpc/windows/arm64"
    "frpc/darwin/amd64"
    "frpc/darwin/arm64"
)

# 默认构建目标（序号，从 1 开始）
DEFAULT_NUMS="1 11 13"

target_label() {
    local target="$1"
    local binary goos goarch goarm suffix arm_label rest
    binary="${target%%/*}"; rest="${target#*/}"
    goos="${rest%%/*}";     rest="${rest#*/}"
    goarch="${rest%%/*}";   goarm="${rest#*/}"
    [ "$goarm" = "$goarch" ] && goarm=""
    suffix=""; [ "$goos" = "windows" ] && suffix=".exe"
    arm_label=""; [ -n "$goarm" ] && arm_label="_v${goarm}"
    echo "${binary}_${goos}_${goarch}${arm_label}${suffix}"
}

build_target() {
    local target="$1"
    local binary goos goarch goarm suffix arm_tag arm_label label rest

    binary="${target%%/*}"; rest="${target#*/}"
    goos="${rest%%/*}";     rest="${rest#*/}"
    goarch="${rest%%/*}";   goarm="${rest#*/}"
    [ "$goarm" = "$goarch" ] && goarm=""

    suffix=""; [ "$goos" = "windows" ] && suffix=".exe"
    arm_tag=""; arm_label=""
    if [ -n "$goarm" ]; then
        arm_tag="GOARM=$goarm"
        arm_label="_v${goarm}"
    fi
    label="${goos}_${goarch}${arm_label}"

    echo "编译 ${binary} -> ${label} ..."
    env CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch $arm_tag \
        go build -trimpath -ldflags "$LDFLAGS" \
        -tags "${binary}${NOWEB_TAG}" \
        -o "${OUTPUT_DIR}/${binary}_${label}${suffix}" \
        ./cmd/${binary}
}

print_targets() {
    echo ""
    echo "  序号  目标名称"
    echo "  ────  ─────────────────────────────"
    local i=1
    for t in "${ALL_TARGETS[@]}"; do
        printf "  %2d    %s\n" "$i" "$(target_label "$t")"
        i=$(( i + 1 ))
    done
    echo "   0    全部"
    echo ""
}

# ── 解析参数 ──────────────────────────────────────
# 支持命令行直接传序号（方便脚本调用）：./build-all-platforms.sh 1 11 13
if [ $# -gt 0 ]; then
    CHOSEN_NUMS=("$@")
else
    # 交互模式：显示列表，用户输入序号
    echo "══════════════════════════════════════════════════"
    echo "  请选择需要构建的目标（输入序号，多个用空格分隔）"
    print_targets
    printf "  默认 [%s]，回车使用默认，输入 0 构建全部: " "$DEFAULT_NUMS"
    read -r input
    if [ -z "$input" ]; then
        input="$DEFAULT_NUMS"
    fi
    echo "══════════════════════════════════════════════════"
    read -ra CHOSEN_NUMS <<< "$input"
fi

# ── 将序号转为目标列表 ────────────────────────────
CHOSEN_TARGETS=()
for num in "${CHOSEN_NUMS[@]}"; do
    if [ "$num" = "0" ]; then
        CHOSEN_TARGETS=("${ALL_TARGETS[@]}")
        break
    fi
    if ! [[ "$num" =~ ^[0-9]+$ ]] || [ "$num" -lt 1 ] || [ "$num" -gt "${#ALL_TARGETS[@]}" ]; then
        echo "错误：无效序号 '$num'，有效范围 1-${#ALL_TARGETS[@]} 或 0（全部）"
        exit 1
    fi
    CHOSEN_TARGETS+=("${ALL_TARGETS[$((num - 1))]}")
done

if [ ${#CHOSEN_TARGETS[@]} -eq 0 ]; then
    echo "未选中任何目标，退出。"
    exit 0
fi

# ── 检查 web 资源 ─────────────────────────────────
if [ ! -d "web/frps/dist" ] || [ ! -d "web/frpc/dist" ]; then
    echo "web 资源未构建，正在执行 make web..."
    make web
    if [ ! -d "web/frps/dist" ] || [ ! -d "web/frpc/dist" ]; then
        NOWEB_TAG=",noweb"
        echo "警告: web 资源构建失败，使用 noweb 标签"
    else
        echo "web 资源构建成功"
    fi
fi

mkdir -p "$OUTPUT_DIR"

echo "即将构建以下目标："
for t in "${CHOSEN_TARGETS[@]}"; do
    echo "  - $(target_label "$t")"
done
echo ""

for t in "${CHOSEN_TARGETS[@]}"; do
    build_target "$t"
done

echo ""
echo "全部编译完成，输出目录: $OUTPUT_DIR"
ls -lh "$OUTPUT_DIR"
