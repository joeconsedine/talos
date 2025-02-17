/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cri

import (
	"context"

	"github.com/pkg/errors"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// PullImage pulls container image
func (c *Client) PullImage(ctx context.Context, image *runtimeapi.ImageSpec, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	resp, err := c.imagesClient.PullImage(ctx, &runtimeapi.PullImageRequest{
		Image:         image,
		SandboxConfig: sandboxConfig,
	})
	if err != nil {
		return "", errors.Wrapf(err, "error pulling image %+v", image)
	}

	return resp.ImageRef, nil
}

// ListImages lists available images
func (c *Client) ListImages(ctx context.Context, filter *runtimeapi.ImageFilter) ([]*runtimeapi.Image, error) {
	resp, err := c.imagesClient.ListImages(ctx, &runtimeapi.ListImagesRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error listing imags")
	}

	return resp.Images, nil
}

// ImageStatus returns the status of the image.
func (c *Client) ImageStatus(ctx context.Context, image *runtimeapi.ImageSpec) (*runtimeapi.Image, error) {
	resp, err := c.imagesClient.ImageStatus(ctx, &runtimeapi.ImageStatusRequest{
		Image: image,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "ImageStatus %q from image service failed", image.Image)
	}

	if resp.Image != nil {
		if resp.Image.Id == "" || resp.Image.Size_ == 0 {
			return nil, errors.Errorf("Id or size of image %q is not set", image.Image)
		}
	}

	return resp.Image, nil
}
