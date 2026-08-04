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
  DeleteApiProV1DocumentFeedbackParams,
  DomainPWResponse,
  DomainResponse,
  GetApiProV1DocumentListParams,
  HandlerV1DocFeedBackLists,
  PostShareProV1DocumentFeedbackPayload,
} from "./types";

/**
 * @description DeleteDocumentFeedbacks
 *
 * @tags documentFeedback
 * @name DeleteApiProV1DocumentFeedback
 * @summary DeleteDocumentFeedbacks
 * @request DELETE:/api/pro/v1/document/feedback
 * @response `200` `DomainResponse` OK
 */

export const deleteApiProV1DocumentFeedback = (
  query: DeleteApiProV1DocumentFeedbackParams,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/api/pro/v1/document/feedback`,
    method: "DELETE",
    query: query,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description GetDocumentFeedbacks
 *
 * @tags documentFeedback
 * @name GetApiProV1DocumentList
 * @summary GetDocumentFeedbacks
 * @request GET:/api/pro/v1/document/list
 * @response `200` `(DomainPWResponse & {
    data?: HandlerV1DocFeedBackLists,

})` OK
 */

export const getApiProV1DocumentList = (
  query: GetApiProV1DocumentListParams,
  params: RequestParams = {},
) =>
  httpRequest<
    DomainPWResponse & {
      data?: HandlerV1DocFeedBackLists;
    }
  >({
    path: `/api/pro/v1/document/list`,
    method: "GET",
    query: query,
    type: ContentType.Json,
    format: "json",
    ...params,
  });

/**
 * @description Create Document Feedback
 *
 * @tags documentFeedback
 * @name PostShareProV1DocumentFeedback
 * @summary Create Document Feedback
 * @request POST:/share/pro/v1/document/feedback
 * @response `200` `DomainResponse` OK
 */

export const postShareProV1DocumentFeedback = (
  data: PostShareProV1DocumentFeedbackPayload,
  params: RequestParams = {},
) =>
  httpRequest<DomainResponse>({
    path: `/share/pro/v1/document/feedback`,
    method: "POST",
    body: data,
    type: ContentType.FormData,
    format: "json",
    ...params,
  });
