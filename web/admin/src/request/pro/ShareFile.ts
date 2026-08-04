/* eslint-disable */
/* tslint:disable */
// @ts-nocheck
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */

import httpRequest, { ContentType, RequestParams } from "./httpClient";
import {
  DomainResponse,
  GithubComChaitinPandaWikiProApiShareV1FileUploadResp,
  PostShareProV1FileUploadPayload,
} from "./types";

/**
 * @description 前台用户上传文件
 *
 * @tags ShareFile
 * @name PostShareProV1FileUpload
 * @summary 文件上传
 * @request POST:/share/pro/v1/file/upload
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiShareV1FileUploadResp,

})` OK
 */

export const postShareProV1FileUpload = (
  data: PostShareProV1FileUploadPayload,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiShareV1FileUploadResp;
    }
  >({
    path: `/share/pro/v1/file/upload`,
    method: "POST",
    body: data,
    type: ContentType.FormData,
    format: "json",
    ...params,
  });
