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
  GithubComChaitinPandaWikiProApiShareV1SubmitContributeReq,
  GithubComChaitinPandaWikiProApiShareV1SubmitContributeResp,
} from "./types";

/**
 * @description 前台用户提交文档编辑或新增贡献
 *
 * @tags ShareContribute
 * @name PostShareProV1ContributeSubmit
 * @summary 提交文档贡献
 * @request POST:/share/pro/v1/contribute/submit
 * @response `200` `(DomainResponse & {
    data?: GithubComChaitinPandaWikiProApiShareV1SubmitContributeResp,

})` OK
 */

export const postShareProV1ContributeSubmit = (
  param: GithubComChaitinPandaWikiProApiShareV1SubmitContributeReq,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainResponse & {
      data?: GithubComChaitinPandaWikiProApiShareV1SubmitContributeResp;
    }
  >({
    path: `/share/pro/v1/contribute/submit`,
    method: "POST",
    body: param,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
