import React from 'react';
import { StyledTopicBox } from '../component/styledCommon';
import { useFadeInText } from '../hooks/useGsapAnimation';

interface TextProps {
  title: string;
}

const Text: React.FC<TextProps> = ({ title }) => {
  const titleRef = useFadeInText(0.2, 0.1);
  return (
    <StyledTopicBox
      ref={titleRef}
      sx={{ fontSize: 60, color: 'text.primary', fontWeight: 700 }}
    >
      {title}
    </StyledTopicBox>
  );
};

export default Text;
