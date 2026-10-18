using System.Text;
using Google.Protobuf;
using Yarn.Compiler;

try {
    using var input = new BinaryReader(Console.OpenStandardInput(), new UTF8Encoding(false, true));
    if (Encoding.ASCII.GetString(input.ReadBytes(4)) != "KPR1") throw new Exception("Unsupported processor request");
    var count = input.ReadUInt32();
    if (count > 256) throw new Exception("Too many inputs");
    var files = new List<Resource>();
    var seen = new HashSet<string>();
    long total = 0;
    for (var n = 0; n < count; n++) {
        var nameSize = input.ReadUInt16();
        var dataSize = input.ReadUInt32();
        total += dataSize;
        if (nameSize == 0 || nameSize > 256 || total > 16 * 1024 * 1024) throw new Exception("Input bounds exceeded");
        var nameBytes = input.ReadBytes(nameSize);
        if (nameBytes.Length != nameSize) throw new Exception("Truncated input name");
        var name = new UTF8Encoding(false, true).GetString(nameBytes);
        if (!seen.Add(name)) throw new Exception("Duplicate input");
        var data = input.ReadBytes((int)dataSize);
        if (data.Length != dataSize) throw new Exception("Truncated input data");
        files.Add(new Resource(name, data));
    }
    if (input.BaseStream.ReadByte() != -1) throw new Exception("Trailing input bytes");
    var job = CompilationJob.CreateFromInputs(files.Select(file =>
        (ISourceInput)new CompilationJob.File { FileName = file.Path, Source = new UTF8Encoding(false, true).GetString(file.Data) }), languageVersion: 3);
    var result = Compiler.Compile(job);
    foreach (var diagnostic in result.Diagnostics) Console.Error.WriteLine(diagnostic);
    if (result.ContainsErrors || result.Program == null || result.StringTable == null) Environment.Exit(1);
    using var tableBody = new MemoryStream();
    using (var table = new BinaryWriter(tableBody, new UTF8Encoding(false, true), true)) {
        void Text(string value, int max) {
            var data = new UTF8Encoding(false, true).GetBytes(value);
            if (data.Length > max) throw new Exception("Yarn string field exceeds bounds");
            table.Write((uint)data.Length); table.Write(data);
        }
        // KST1 sorts UTF-8 bytes, including IDs outside the BMP.
        var utf8Order = Comparer<string>.Create((a, b) =>
            Encoding.UTF8.GetBytes(a).AsSpan().SequenceCompareTo(Encoding.UTF8.GetBytes(b)));
        var rows = result.StringTable!.OrderBy(item => item.Key, utf8Order).ToArray();
        if (rows.Length > 65536) throw new Exception("Yarn string count exceeds bounds");
        table.Write((uint)rows.Length);
        foreach (var item in rows) {
            Text(item.Key, 256); Text(item.Value.text, 4096);
            var tags = item.Value.metadata.SelectMany(value => value.Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries)).ToArray();
            if (tags.Length > 128) throw new Exception("Yarn metadata count exceeds bounds");
            table.Write((uint)tags.Length); foreach (var tag in tags) Text(tag, 256);
        }
    }
    if (tableBody.Length > 4 * 1024 * 1024 - 8) throw new Exception("Yarn strings exceed bounds");
    using var tableRecord = new MemoryStream();
    using (var record = new BinaryWriter(tableRecord, Encoding.UTF8, true)) {
        record.Write(Encoding.ASCII.GetBytes("KST1")); record.Write((uint)tableBody.Length); record.Write(tableBody.ToArray());
    }
    Resource[] response = [
        new Resource("story.yarnc", result.Program!.ToByteArray()),
        new Resource("strings.bin", tableRecord.ToArray()),
    ];
    using var output = new BinaryWriter(Console.OpenStandardOutput());
    output.Write(Encoding.ASCII.GetBytes("KPR1"));
    output.Write((uint)response.Length);
    foreach (var file in response) {
        var name = Encoding.UTF8.GetBytes(file.Path);
        output.Write((ushort)name.Length);
        output.Write((uint)file.Data.Length);
        output.Write(name);
        output.Write(file.Data);
    }
} catch (Exception error) { Console.Error.WriteLine(error.Message); Environment.Exit(1); }

record Resource(string Path, byte[] Data);
