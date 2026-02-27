require 'json'

payload = JSON.parse($stdin.read)
text    = payload['text']

# Handle YAML frontmatter — convert to bold key: value display
if text.start_with?("---\n")
  fm_end = text.index("\n---\n", 4) || text.index("\n...\n", 4)
  if fm_end
    fm_content = text[4...fm_end]
    rest = text[(fm_end + 5)..]
    fm_display = fm_content.each_line.map do |line|
      if (m = line.match(/\A([^:\n]+):\s*(.*)\n?\z/))
        "**#{m[1]}:**{.fmkey} #{m[2]}  \n"
      else
        line
      end
    end.join
    text = "\\-\\-\\-\n" + fm_display + "\\-\\-\\-\n\n" + rest
  end
end

fence_indent = nil
fence_ticks  = nil

# Single pass: plain-text output (no list conversion)
# Project names get bold + color; tasks get escaped dash; @tags handled by atag plugin
result = text.each_line.map do |line|
  if fence_indent.nil?
    if (m = line.match(/\A(\s*)(`{3,})/))
      fence_indent = m[1].length
      fence_ticks  = m[2].length
      next line
    end
  else
    if (m = line.match(/\A(\s*)(`{3,})\s*\z/)) &&
        m[1].length <= fence_indent && m[2].length >= fence_ticks
      fence_indent = nil
      fence_ticks  = nil
    end
    next line
  end

  if (m = line.match(/\A(\t*)[^\s#>].*:\s*\z/)) && !line.match?(/https?:/)
    tabs   = m[1].length
    indent = "\u00A0\u00A0" * [tabs - 1, 0].max
    cls    = 'h4text'
    "#{indent}**#{line.sub(/\A\t*/, '').rstrip}**{.#{cls}}  \n"
  elsif line.match?(/\A\t+- /)
    tabs   = line.index('-')
    indent = "\u00A0\u00A0" * [tabs - 1, 0].max
    "#{indent}\\- #{line.sub(/\A\t+- /, '').rstrip}  \n"
  elsif line.match?(/\A {4}/)
    spaces = line[/\A */].length
    level  = spaces / 4
    rest   = line.sub(/\A +/, '')
    rest.start_with?("- ") \
      ? "#{"\u00A0\u00A0" * [level - 1, 0].max}\\- #{rest[2..].rstrip}  \n" \
      : line
  elsif (m = line.chomp.match(/\A(##?) (.+)\z/))
    hashes  = m[1]
    content = m[2].rstrip
    prefix  = '\\#' * hashes.length
    "#{hashes} **#{prefix} #{content}**\n"
  elsif (m = line.chomp.match(/\A(####?) (.+)\z/))
    prefix = '\\#' * m[1].length
    "**#{prefix} #{m[2].rstrip}**{.h3text}  \n"
  else
    line
  end
end.join

print result
