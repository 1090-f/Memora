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
import { DomainCommentModerateListReq, DomainResponse } from "./types";

/**
 * @description BatchModerateComment
 *
 * @tags comment
 * @name PostApiProV1CommentModerate
 * @summary BatchModerateComment
 * @request POST:/api/pro/v1/comment_moderate
 * @response `200` `DomainResponse` success
 */

export const postApiProV1CommentModerate = (
  req: DomainCommentModerateListReq,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/pro/v1/comment_moderate`,
    method: "POST",
    body: req,
    type: ContentType.Json,
    format: "json",
    ...params,
  });
