package compression

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

func Tar(srcDir, tarFile string) error {
    out, err := os.Create(tarFile)
    if err != nil {
        return err
    }
    defer out.Close()

    gz := gzip.NewWriter(out)
	defer gz.Close()

	tw := tar.NewWriter(gz)
    defer tw.Close()

    return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        header, err := tar.FileInfoHeader(info, "")
        if err != nil {
            return err
        }

        relPath, err := filepath.Rel(srcDir, path)
        if err != nil {
            return err
        }

        header.Name = relPath

        if err := tw.WriteHeader(header); err != nil {
            return err
        }

        if info.IsDir() {
            return nil
        }

        file, err := os.Open(path)
        if err != nil {
            return err
        }
        defer file.Close()

        _, err = io.Copy(tw, file)
        return err
    })
}

func Untar(tarFile, dest string) error {
    in, err := os.Open(tarFile)
    if err != nil {
        return err
    }
    defer in.Close()

    gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

    for {
        hdr, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        target := filepath.Join(dest, hdr.Name)

        switch hdr.Typeflag {
        case tar.TypeDir:
            if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
                return err
            }

        case tar.TypeReg:
            if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
                return err
            }

            f, err := os.OpenFile(target,
                os.O_CREATE|os.O_RDWR|os.O_TRUNC,
                os.FileMode(hdr.Mode))
            if err != nil {
                return err
            }

            if _, err := io.Copy(f, tr); err != nil {
                f.Close()
                return err
            }
            f.Close()
        }
    }

    return nil
}